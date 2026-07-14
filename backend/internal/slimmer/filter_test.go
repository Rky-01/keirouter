package slimmer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseFilterLevel(t *testing.T) {
	cases := map[string]FilterLevel{
		"none":       FilterNone,
		"minimal":    FilterMinimal,
		"aggressive": FilterAggressive,
		"":           FilterNone,
		"bogus":      FilterNone,
		" Minimal ":  FilterMinimal,
	}
	for input, want := range cases {
		got := ParseFilterLevel(input)
		require.Equal(t, want, got, "ParseFilterLevel(%q)", input)
	}
}

func TestDetectLanguage(t *testing.T) {
	cases := map[string]Language{
		`{"key": "value"}`:                   LangData,
		"---\nname: test\nvalue: 42\n":       LangData,
		"package main\n\nfunc main() {\n}\n": LangCFamily,
		"def hello():\n    return True\n":    LangPython,
		"#!/bin/bash\necho hello\n":          LangRubyShell,
		"func main() {\n\treturn nil\n}\n":   LangCFamily,
	}
	for input, want := range cases {
		got := detectLanguage(input)
		require.Equal(t, want, got, "detectLanguage(%q)", input[:min(30, len(input))])
	}
}

func TestStripCFamilyComments_Minimal(t *testing.T) {
	input := `package main

// This comment should be removed
func main() {
	// Another comment
	fmt.Println("hello") // inline comment
}
`
	result, err := stripComments(input, FilterMinimal)
	require.NoError(t, err)
	require.NotContains(t, result, "This comment should be removed")
	require.NotContains(t, result, "Another comment")
	require.Contains(t, result, `fmt.Println("hello")`)
	require.Contains(t, result, "func main()")
}

func TestStripCFamilyComments_BlockComment(t *testing.T) {
	input := `package main

/*
 * This is a block comment
 * spanning multiple lines
 */
func main() {
	x := 1 /* inline block */ + 2
}
`
	result, err := stripComments(input, FilterMinimal)
	require.NoError(t, err)
	require.NotContains(t, result, "block comment")
	require.Contains(t, result, "func main()")
}

func TestStripPythonComments_Minimal(t *testing.T) {
	input := `import os

# This comment should go
def hello():
    # Another comment
    return True  # inline comment
`
	result, err := stripComments(input, FilterMinimal)
	require.NoError(t, err)
	require.NotContains(t, result, "This comment should go")
	require.NotContains(t, result, "Another comment")
	require.Contains(t, result, "def hello()")
	require.Contains(t, result, "return True")
}

// Aggressive mode performs signature extraction: imports and signatures survive
// while bodies — including docstrings — collapse to a single
// "// ... implementation" marker.
func TestStripPythonComments_AggressiveDocstrings(t *testing.T) {
	input := `import os

def hello():
    """This docstring should be removed in aggressive mode."""
    return True

class Foo:
    """
    Multi-line docstring
    that should also go.
    """
    pass
`
	result, err := stripComments(input, FilterAggressive)
	require.NoError(t, err)
	require.NotContains(t, result, "docstring should be removed")
	require.NotContains(t, result, "Multi-line docstring")
	require.Contains(t, result, "def hello()")
	require.Contains(t, result, "class Foo")
	require.Contains(t, result, "// ... implementation")
	// Body statements are collapsed, not preserved, in aggressive mode.
	require.NotContains(t, result, "return True")
}

// TestAggressiveSignatureExtraction verifies aggressive mode keeps the
// structural skeleton (imports + signatures) and collapses function bodies.
func TestAggressiveSignatureExtraction(t *testing.T) {
	code := `package main

import "fmt"

func greet(name string) string {
	msg := "hello " + name
	return msg
}
`
	result, err := stripComments(code, FilterAggressive)
	require.NoError(t, err)
	require.Contains(t, result, `import "fmt"`)
	require.Contains(t, result, "func greet")
	require.Contains(t, result, "// ... implementation")
	require.NotContains(t, result, `msg := "hello "`)
	require.LessOrEqual(t, len(result), len(code))
}

// TestMinimalKeepsDocComments verifies minimal mode preserves doc comments
// (///) while dropping ordinary line comments.
func TestMinimalKeepsDocComments(t *testing.T) {
	code := "package main\n\n// ordinary comment to drop\n/// doc comment to keep\nfunc main() {\n\treturn\n}\n"
	result, err := stripComments(code, FilterMinimal)
	require.NoError(t, err)
	require.NotContains(t, result, "ordinary comment to drop")
	require.Contains(t, result, "/// doc comment to keep")
}

func TestStripHashComments(t *testing.T) {
	input := `#!/bin/bash

# This is a comment
echo "hello" # inline
ls -la
`
	result, err := stripComments(input, FilterMinimal)
	require.NoError(t, err)
	require.Contains(t, result, "#!/bin/bash")
	require.NotContains(t, result, "This is a comment")
	require.Contains(t, result, `echo "hello"`)
}

func TestNormalizeBlankLines(t *testing.T) {
	input := "package main\n\n\nimport \"fmt\"\n\n\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"
	result := normalizeBlankLines(input)
	lines := strings.Split(result, "\n")
	consecutiveBlanks := 0
	for _, l := range lines {
		if l == "" {
			consecutiveBlanks++
			require.LessOrEqual(t, consecutiveBlanks, 1, "runs of blank lines must collapse")
		} else {
			consecutiveBlanks = 0
		}
	}
}

// TestSmartTruncate verifies there are no synthetic comment annotations, a
// single "[N more lines]" marker, and the invariant that the kept line count
// plus the reported overflow equals the input line count.
func TestSmartTruncate(t *testing.T) {
	const total, maxLines = 200, 20
	rows := make([]string, total)
	for i := 0; i < total; i++ {
		rows[i] = fmt.Sprintf("plain text line number %d", i)
	}
	out := smartTruncate(strings.Join(rows, "\n"), maxLines)

	require.NotContains(t, out, "// ...", "must not emit comment-like markers")

	var overflow string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "more lines") {
			overflow = strings.TrimSpace(l)
		}
	}
	require.NotEmpty(t, overflow, "overflow marker missing")

	var reported int
	_, err := fmt.Sscanf(overflow, "[%d more lines]", &reported)
	require.NoError(t, err)

	kept := 0
	for _, l := range strings.Split(out, "\n") {
		if !strings.Contains(l, "more lines") {
			kept++
		}
	}
	require.Equal(t, total, kept+reported, "kept + overflow must equal total")
}

func TestSmartTruncate_NoTruncationUnderLimit(t *testing.T) {
	in := "a\nb\nc"
	require.Equal(t, in, smartTruncate(in, 10))
}

func TestReducedCap(t *testing.T) {
	require.Equal(t, 5, reduced(CapWarnings, 5)) // 10 - 5
	require.Equal(t, 15, reduced(CapList, 5))    // 20 - 5
	require.Equal(t, 4, reduced(4, 5))           // by >= cap: fall back to cap
	require.Equal(t, 0, reduced(0, 5))           // zero cap stays zero
}

func TestSourceCodeRule_Detect(t *testing.T) {
	r := sourceCodeRule{}

	code := "package main\n\n// comment here\nfunc main() {\n\treturn nil\n}\n"
	require.Greater(t, r.Detect(code), 0.0, "should detect Go source")

	data := `{"key": "value", "nested": {"a": 1}}`
	require.Equal(t, 0.0, r.Detect(data), "should not detect JSON data")
}

func TestStripComments_NoGrowSafety(t *testing.T) {
	tiny := "x := 1\n"
	result, err := stripComments(tiny, FilterMinimal)
	require.NoError(t, err)
	require.LessOrEqual(t, len(result), len(tiny))
}
