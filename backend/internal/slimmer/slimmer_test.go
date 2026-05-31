package slimmer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/stretchr/testify/require"
)

// toolResultReq builds a request whose single message carries one tool result.
func toolResultReq(content string, isErr bool) *core.ChatRequest {
	return &core.ChatRequest{
		Messages: []core.Message{{
			Role: core.RoleTool,
			Content: []core.ContentPart{{
				Type:       core.PartToolResult,
				ToolResult: &core.ToolResult{CallID: "c1", Content: content, IsError: isErr},
			}},
		}},
	}
}

func resultContent(req *core.ChatRequest) string {
	return req.Messages[0].Content[0].ToolResult.Content
}

func TestEngine_DisabledIsNoop(t *testing.T) {
	big := strings.Repeat("line\n", 500)
	req := toolResultReq(big, false)
	stats := Default().Compress(req, Config{Enabled: false})
	require.Nil(t, stats)
	require.Equal(t, big, resultContent(req))
}

func TestEngine_SkipsErrorResults(t *testing.T) {
	// A diff that would normally compress, but flagged as an error result.
	diff := "diff --git a/x b/x\n" + strings.Repeat("@@ -1,1 +1,1 @@\n context\n", 200)
	req := toolResultReq(diff, true)
	stats := Default().Compress(req, Config{Enabled: true})
	require.Nil(t, stats, "error results must be left untouched")
	require.Equal(t, diff, resultContent(req))
}

func TestEngine_SkipsSmallPayloads(t *testing.T) {
	req := toolResultReq("tiny output", false)
	stats := Default().Compress(req, Config{Enabled: true})
	require.Nil(t, stats)
}

func TestEngine_NeverGrowsContent(t *testing.T) {
	// Whatever rule fires, output must be <= input.
	inputs := []string{
		"diff --git a/f b/f\n" + strings.Repeat("@@ -0,0 +1 @@\n+x\n", 300),
		strings.Repeat("src/a/b/c.go\n", 400),
		buildGrepBlob(),
	}
	for _, in := range inputs {
		req := toolResultReq(in, false)
		Default().Compress(req, Config{Enabled: true})
		require.LessOrEqual(t, len(resultContent(req)), len(in))
	}
}

func TestGrepRule_CapsPerFile(t *testing.T) {
	blob := buildGrepBlob()
	req := toolResultReq(blob, false)
	stats := Default().Compress(req, Config{Enabled: true})
	require.NotNil(t, stats)
	require.Equal(t, "grep", stats.Hits[0].Rule)

	out := resultContent(req)
	require.Contains(t, out, "more matches")
	// No more than grepPerFileMax raw match lines per file should survive.
	count := strings.Count(out, "app.go:")
	require.LessOrEqual(t, count, grepPerFileMax+1) // +1 for the summary line
}

func TestGitDiffRule_PreservesHeadersAndChanges(t *testing.T) {
	var b strings.Builder
	b.WriteString("diff --git a/main.go b/main.go\n")
	b.WriteString("@@ -1,200 +1,200 @@\n")
	for i := 0; i < 180; i++ {
		fmt.Fprintf(&b, " unchanged context %d\n", i)
	}
	b.WriteString("+added critical line\n")
	b.WriteString("-removed critical line\n")
	req := toolResultReq(b.String(), false)

	stats := Default().Compress(req, Config{Enabled: true})
	require.NotNil(t, stats)
	out := resultContent(req)
	require.Contains(t, out, "diff --git a/main.go b/main.go")
	require.Contains(t, out, "+added critical line")
	require.Contains(t, out, "-removed critical line")
	require.Contains(t, out, "context lines elided")
}

func TestDedupRule_CollapsesRepeats(t *testing.T) {
	// A long, path-free, colon-free repeated line so no structured rule
	// (ls/find/grep/tree) claims it and dedup deterministically wins.
	line := "this is a fairly long repeated log message that exceeds eighty characters in total length\n"
	blob := strings.Repeat(line, 200)
	req := toolResultReq(blob, false)
	stats := Default().Compress(req, Config{Enabled: true})
	require.NotNil(t, stats)
	require.Equal(t, "dedup-log", stats.Hits[0].Rule)
	require.Less(t, len(resultContent(req)), len(blob))
	require.Contains(t, resultContent(req), "×")
}

func TestEngine_DisabledRuleSkipped(t *testing.T) {
	blob := buildGrepBlob()
	req := toolResultReq(blob, false)
	// Disable grep; some other generic rule may still fire, but not grep.
	stats := Default().Compress(req, Config{Enabled: true, Disabled: []string{"grep"}})
	if stats != nil {
		for _, h := range stats.Hits {
			require.NotEqual(t, "grep", h.Rule)
		}
	}
}

func TestStats_Saved(t *testing.T) {
	s := Stats{BytesBefore: 1000, BytesAfter: 400}
	require.Equal(t, 600, s.Saved())
}

func buildGrepBlob() string {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "app.go:%d: match content here number %d\n", i+1, i)
	}
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "util.go:%d: another match %d\n", i+1, i)
	}
	return b.String()
}