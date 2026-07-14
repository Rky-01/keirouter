package slimmer

import (
	"fmt"
	"regexp"
	"strings"
)

// FilterLevel selects how aggressively source code comments are stripped.
//
// The levels are:
//   - Minimal strips full-line and block comments but preserves doc comments
//     (///, /**, Python docstrings) and never touches inline comments, then
//     normalizes runs of blank lines down to at most two.
//   - Aggressive first runs the minimal pass, then reduces source down to its
//     structural skeleton: imports, function/type signatures, and top-level
//     constants are kept while bodies collapse to a "// ... implementation"
//     marker.
type FilterLevel int

const (
	// FilterNone disables source code filtering (default, backward compatible).
	FilterNone FilterLevel = iota
	// FilterMinimal strips comments (keeping doc comments) and normalizes blanks.
	FilterMinimal
	// FilterAggressive keeps only the structural skeleton (signatures + imports).
	FilterAggressive
)

// ParseFilterLevel converts a settings string to a FilterLevel. Unrecognized
// values default to FilterNone for safety.
func ParseFilterLevel(s string) FilterLevel {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "minimal":
		return FilterMinimal
	case "aggressive":
		return FilterAggressive
	default:
		return FilterNone
	}
}

// Language identifies a programming language detected from tool-result content.
type Language int

const (
	LangUnknown Language = iota
	LangCFamily          // Go, JS, TS, Rust, Java, C, C++
	LangPython
	LangRubyShell // Ruby, Shell — #-style comments only
	LangData      // JSON, YAML, TOML, XML — no comment stripping
)

// commentPatterns describes the comment syntax for a language group.
// Empty strings mean "not applicable".
type commentPatterns struct {
	line       string // single-line comment marker (e.g. "//", "#")
	blockStart string // block comment opener (e.g. "/*")
	blockEnd   string // block comment closer (e.g. "*/")
	docLine    string // preserved line-doc marker (e.g. "///")
	docBlock   string // preserved block-doc opener (e.g. "/**", `"""`)
}

// patterns returns the comment syntax for the language group.
func (l Language) patterns() commentPatterns {
	switch l {
	case LangCFamily:
		return commentPatterns{line: "//", blockStart: "/*", blockEnd: "*/", docLine: "///", docBlock: "/**"}
	case LangPython:
		return commentPatterns{line: "#", blockStart: `"""`, blockEnd: `"""`, docBlock: `"""`}
	case LangRubyShell:
		return commentPatterns{line: "#", blockStart: "=begin", blockEnd: "=end"}
	case LangData:
		return commentPatterns{} // never strip data formats
	default: // LangUnknown — conservative C-family defaults
		return commentPatterns{line: "//", blockStart: "/*", blockEnd: "*/"}
	}
}

// detectLanguage guesses the language from syntax patterns in the content probe.
// Returns LangData when the content looks like a data format (no stripping).
func detectLanguage(content string) Language {
	probe := content
	if len(probe) > 2048 {
		probe = probe[:2048]
	}

	// Data formats — never strip comments.
	if looksLikeData(probe) {
		return LangData
	}

	// Python indicators.
	if strings.Contains(probe, "def ") || strings.Contains(probe, "import ") ||
		strings.Contains(probe, "class ") && strings.Contains(probe, ":") ||
		strings.Contains(probe, `"""`) {
		return LangPython
	}

	// Ruby/Shell indicators (must check before C-family since # comments overlap).
	if strings.Contains(probe, "require ") && strings.Contains(probe, "end\n") {
		return LangRubyShell
	}
	if strings.HasPrefix(probe, "#!/bin/") || strings.HasPrefix(probe, "#!/usr/bin/") ||
		strings.Contains(probe, "then\n") || strings.Contains(probe, "fi\n") {
		return LangRubyShell
	}

	// C-family indicators.
	if strings.Contains(probe, "func ") || strings.Contains(probe, "fn ") ||
		strings.Contains(probe, "package ") || strings.Contains(probe, "import ") ||
		strings.Contains(probe, "class ") || strings.Contains(probe, "interface ") ||
		strings.Contains(probe, "function ") || strings.Contains(probe, "const ") ||
		strings.Contains(probe, "let ") || strings.Contains(probe, "var ") {
		return LangCFamily
	}

	return LangUnknown
}

var reDataFormat = regexp.MustCompile(`(?m)^\s*("[\w-]+"\s*:|[<{[]\s*$|\w+\s*=\s*["{[]|---\s*$)`)

func looksLikeData(probe string) bool {
	// JSON/YAML/TOML indicators.
	trimmed := strings.TrimSpace(probe)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return true
	}
	if strings.Contains(trimmed, "---\n") { // YAML document separator
		return true
	}
	matches := reDataFormat.FindAllString(probe, -1)
	return len(matches) > len(strings.Split(probe, "\n"))/4
}

// sourceCodeRule strips comments from source code in tool results.
// It only fires when no other rule matches (low detect score) and the content
// looks like source code.
type sourceCodeRule struct{}

func (sourceCodeRule) Name() string { return "source-code" }

func (sourceCodeRule) Detect(probe string) float64 {
	if looksLikeData(probe) {
		return 0
	}

	// Look for comment syntax and code keywords.
	hasComments := strings.Contains(probe, "//") || strings.Contains(probe, "/*") ||
		strings.Contains(probe, "# ")
	hasCode := strings.Contains(probe, "func ") || strings.Contains(probe, "def ") ||
		strings.Contains(probe, "fn ") || strings.Contains(probe, "class ") ||
		strings.Contains(probe, "import ") || strings.Contains(probe, "package ") ||
		strings.Contains(probe, "return ") || strings.Contains(probe, "if ")

	if hasComments && hasCode {
		return 0.55
	}
	return 0
}

// Compress strips comments from source code content. When called directly it
// uses FilterMinimal; the engine applies aggressive filtering as a secondary
// pass when configured.
func (sourceCodeRule) Compress(content string) (string, error) {
	return stripComments(content, FilterMinimal)
}

// ---- comment stripping ------------------------------------------------------

var (
	// reMultiBlank collapses runs of 3+ newlines to a single blank line.
	reMultiBlank = regexp.MustCompile(`\n{3,}`)

	// reImportLine matches import-like statements that aggressive mode preserves.
	reImportLine = regexp.MustCompile(`^(use |import |from |require\(|#include)`)

	// reFuncSig matches function/type declaration signatures preserved by
	// aggressive mode (and treated as "important" by smartTruncate).
	reFuncSig = regexp.MustCompile(`^(pub\s+)?(async\s+)?(fn|def|function|func|class|struct|enum|trait|interface|type)\s+\w+`)
)

// stripComments removes comments from content based on the detected language and
// filter level. Data formats (JSON/YAML/TOML/...) are returned untouched so that
// constructs like "packages/*" are never mistaken for block comments. The result
// is never returned larger than the input.
func stripComments(content string, level FilterLevel) (string, error) {
	if level == FilterNone {
		return content, nil
	}

	lang := detectLanguage(content)
	if lang == LangData {
		return content, nil
	}

	minimal := normalizeBlankLines(minimalFilter(content, lang.patterns()))

	result := minimal
	if level == FilterAggressive {
		result = aggressiveFilter(minimal)
	}

	// Safety: never return larger content, and never empty a payload.
	if len(result) >= len(content) || result == "" {
		return content, nil
	}
	return result, nil
}

// minimalFilter drops full-line comments and block comments while preserving doc
// comments, shebangs, and inline comments.
func minimalFilter(content string, p commentPatterns) string {
	var b strings.Builder
	b.Grow(len(content))
	inBlock := false
	inDocstring := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)

		// Preserve shebangs (also used as a language signal).
		if strings.HasPrefix(trimmed, "#!") {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}

		// Block comments (skipped unless they open a preserved doc block).
		if p.blockStart != "" && p.blockEnd != "" {
			isDocBlock := p.docBlock != "" && strings.HasPrefix(trimmed, p.docBlock)
			if !inDocstring && strings.Contains(trimmed, p.blockStart) && !isDocBlock {
				inBlock = true
			}
			if inBlock {
				if strings.Contains(trimmed, p.blockEnd) {
					inBlock = false
				}
				continue
			}
		}

		// Python-style docstrings are preserved in minimal mode.
		if p.docBlock == `"""` && strings.HasPrefix(trimmed, `"""`) {
			inDocstring = !inDocstring
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}
		if inDocstring {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}

		// Full-line comments are dropped, except doc comments (e.g. ///).
		if p.line != "" && strings.HasPrefix(trimmed, p.line) {
			if p.docLine != "" && strings.HasPrefix(trimmed, p.docLine) {
				b.WriteString(line)
				b.WriteByte('\n')
			}
			continue
		}

		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// aggressiveFilter reduces already-minimal source to its structural skeleton:
// imports, signatures, and top-level constants are kept; bodies collapse to a
// single "// ... implementation" marker.
func aggressiveFilter(minimal string) string {
	var b strings.Builder
	braceDepth := 0
	inImplBody := false

	for _, line := range strings.Split(minimal, "\n") {
		trimmed := strings.TrimSpace(line)

		// Always keep imports.
		if reImportLine.MatchString(trimmed) {
			b.WriteString(line)
			b.WriteByte('\n')
			continue
		}

		// Always keep function/type signatures.
		if reFuncSig.MatchString(trimmed) {
			b.WriteString(line)
			b.WriteByte('\n')
			inImplBody = true
			braceDepth = 0
			continue
		}

		openBraces := strings.Count(trimmed, "{")
		closeBraces := strings.Count(trimmed, "}")

		if inImplBody {
			braceDepth += openBraces - closeBraces

			// Keep only the outermost braces of the body.
			if braceDepth <= 1 && (trimmed == "{" || trimmed == "}" || strings.HasSuffix(trimmed, "{")) {
				b.WriteString(line)
				b.WriteByte('\n')
			}

			if braceDepth <= 0 {
				inImplBody = false
				if trimmed != "" && trimmed != "}" {
					b.WriteString("    // ... implementation\n")
				}
			}
			continue
		}

		// Keep top-level constants and statics.
		if strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "static ") ||
			strings.HasPrefix(trimmed, "let ") || strings.HasPrefix(trimmed, "pub const ") ||
			strings.HasPrefix(trimmed, "pub static ") {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}

	return strings.TrimSpace(b.String())
}

// normalizeBlankLines collapses runs of three or more newlines to a single
// blank line and trims surrounding whitespace.
func normalizeBlankLines(s string) string {
	s = reMultiBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// smartTruncate keeps a useful window of a long payload: structurally important
// lines (signatures, imports, braces, exported items) are always kept, plus up
// to maxLines/2 leading ordinary lines. The omitted tail is summarized with a
// single unambiguous "[N more lines]" marker rather than inline comment-like
// markers that agents can mistake for code.
func smartTruncate(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}

	result := make([]string, 0, maxLines+1)
	kept := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		important := reFuncSig.MatchString(trimmed) ||
			reImportLine.MatchString(trimmed) ||
			strings.HasPrefix(trimmed, "pub ") ||
			strings.HasPrefix(trimmed, "export ") ||
			trimmed == "}" || trimmed == "{"

		if important || kept < maxLines/2 {
			result = append(result, line)
			kept++
		}
		if kept >= maxLines-1 {
			break
		}
	}

	result = append(result, fmt.Sprintf("[%d more lines]", len(lines)-kept))
	return strings.Join(result, "\n")
}
