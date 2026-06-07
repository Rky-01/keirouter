package transform

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
)

func TestKiro_RenderRequest_ConversationState(t *testing.T) {
	req := &core.ChatRequest{
		Model:  "claude-sonnet-4.5",
		System: "be precise",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "hello"}}},
		},
	}
	body, err := KiroCodec{}.RenderRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	cs, ok := env["conversationState"].(map[string]any)
	if !ok {
		t.Fatal("missing conversationState")
	}
	cm := cs["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	content := cm["content"].(string)
	// System folds into the user content; a context marker is prepended.
	if !strings.Contains(content, "be precise") {
		t.Errorf("system should fold into content: %q", content)
	}
	if !strings.Contains(content, "[Context: Current time") {
		t.Errorf("context marker missing: %q", content)
	}
	if cm["modelId"] != "claude-sonnet-4.5" {
		t.Errorf("modelId wrong: %v", cm["modelId"])
	}
}

func TestKiro_RenderRequest_ThinkingSuffix(t *testing.T) {
	// The synthetic -thinking suffix injects the thinking_mode prefix and is
	// stripped from the upstream modelId.
	req := &core.ChatRequest{
		Model: "claude-sonnet-4.5-thinking",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "hi"}}},
		},
	}
	body, err := KiroCodec{}.RenderRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	cm := env["conversationState"].(map[string]any)["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	content := cm["content"].(string)
	if !strings.Contains(content, "<thinking_mode>enabled</thinking_mode>") {
		t.Errorf("thinking prefix missing: %q", content)
	}
	if cm["modelId"] != "claude-sonnet-4.5" {
		t.Errorf("upstream modelId should strip -thinking, got %v", cm["modelId"])
	}
}

func TestKiro_RenderRequest_ToolsAndHistory(t *testing.T) {
	req := &core.ChatRequest{
		Model: "claude-sonnet-4.5",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "first"}}},
			{Role: core.RoleAssistant, Content: []core.ContentPart{{Type: core.PartText, Text: "ok"}}},
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "second"}}},
		},
		Tools: []core.Tool{{Name: "get_weather", Description: "weather", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}
	body, err := KiroCodec{}.RenderRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	cs := env["conversationState"].(map[string]any)

	// History holds the earlier user + assistant turns; the last user message is
	// promoted to currentMessage.
	history := cs["history"].([]any)
	if len(history) < 2 {
		t.Fatalf("expected history with prior turns, got %d", len(history))
	}

	cm := cs["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	if !strings.Contains(cm["content"].(string), "second") {
		t.Errorf("current message should be the last user turn: %v", cm["content"])
	}
	// Tools attach to the current message context.
	ctx, ok := cm["userInputMessageContext"].(map[string]any)
	if !ok || ctx["tools"] == nil {
		t.Errorf("tools should attach to current message context: %v", cm)
	}
}

// When the client sends NO tools but the history references tool calls/results,
// the structured tool content must be flattened to text. Leaving structured
// tool references without a tools array makes Kiro return 400 "Improperly
// formed request".
func TestKiro_RenderRequest_FlattensToolsWhenClientSentNone(t *testing.T) {
	req := &core.ChatRequest{
		Model: "claude-opus-4.8-thinking",
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "list files"}}},
			{Role: core.RoleAssistant, Content: []core.ContentPart{
				{Type: core.PartText, Text: "calling tool"},
				{Type: core.PartToolCall, ToolCall: &core.ToolCall{ID: "call_1", Name: "ls", Arguments: json.RawMessage(`{"path":"."}`)}},
			}},
			{Role: core.RoleTool, Content: []core.ContentPart{
				{Type: core.PartToolResult, ToolResult: &core.ToolResult{CallID: "call_1", Content: "a.go\nb.go"}},
			}},
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "thanks"}}},
		},
		// No Tools sent by the client.
	}
	body, err := KiroCodec{}.RenderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	// No structured tool content (toolUses/toolResults/tools) may survive
	// anywhere in the payload.
	raw := string(body)
	for _, banned := range []string{`"toolUses"`, `"toolResults"`, `"toolSpecification"`, `"tools"`} {
		if strings.Contains(raw, banned) {
			t.Errorf("flattened payload must not contain %s: %s", banned, raw)
		}
	}

	// The tool call/result content should survive as readable text in history.
	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	cs := env["conversationState"].(map[string]any)
	var allText strings.Builder
	for _, h := range cs["history"].([]any) {
		hm := h.(map[string]any)
		if uim, ok := hm["userInputMessage"].(map[string]any); ok {
			allText.WriteString(uim["content"].(string))
			allText.WriteString("\n")
		}
		if arm, ok := hm["assistantResponseMessage"].(map[string]any); ok {
			allText.WriteString(arm["content"].(string))
			allText.WriteString("\n")
		}
	}
	// The last user turn is promoted to currentMessage, so the flattened tool
	// result (folded into the final user turn) lives there.
	cm := cs["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	allText.WriteString(cm["content"].(string))
	text := allText.String()
	if !strings.Contains(text, "[Tool call: ls(") {
		t.Errorf("tool call should be flattened to text: %q", text)
	}
	if !strings.Contains(text, "[Tool result: a.go") {
		t.Errorf("tool result should be flattened to text: %q", text)
	}
}

// When the client DOES send tools but a tool_result references a tool_use that
// was dropped by client-side compaction, the orphaned result must be folded
// back into user text instead of left as a dangling structured reference
// (which makes Kiro return 400).
func TestKiro_RenderRequest_ReconcilesOrphanedToolResults(t *testing.T) {
	req := &core.ChatRequest{
		Model: "claude-opus-4.8-thinking",
		Messages: []core.Message{
			// Assistant message that WOULD have contained the matching tool_use
			// has been compacted away — only its text remains.
			{Role: core.RoleAssistant, Content: []core.ContentPart{{Type: core.PartText, Text: "earlier reply"}}},
			// Orphaned tool result: no assistant toolUse has id "orphan_1".
			{Role: core.RoleTool, Content: []core.ContentPart{
				{Type: core.PartToolResult, ToolResult: &core.ToolResult{CallID: "orphan_1", Content: "stale output"}},
			}},
			{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: "continue please"}}},
		},
		Tools: []core.Tool{{Name: "noop", Description: "noop", Parameters: json.RawMessage(`{"type":"object"}`)}},
	}
	body, err := KiroCodec{}.RenderRequest(req)
	if err != nil {
		t.Fatal(err)
	}

	var env map[string]any
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatal(err)
	}
	cs := env["conversationState"].(map[string]any)

	// The orphaned toolResult must not survive as a structured reference.
	walk := func(uim map[string]any) {
		if ctx, ok := uim["userInputMessageContext"].(map[string]any); ok {
			if trs, ok := ctx["toolResults"].([]any); ok {
				for _, tr := range trs {
					if id, _ := tr.(map[string]any)["toolUseId"].(string); id == "orphan_1" {
						t.Errorf("orphaned toolResult should be removed, found: %v", tr)
					}
				}
			}
		}
	}
	collected := strings.Builder{}
	for _, h := range cs["history"].([]any) {
		if uim, ok := h.(map[string]any)["userInputMessage"].(map[string]any); ok {
			walk(uim)
			collected.WriteString(uim["content"].(string))
			collected.WriteString("\n")
		}
	}
	cm := cs["currentMessage"].(map[string]any)["userInputMessage"].(map[string]any)
	walk(cm)
	collected.WriteString(cm["content"].(string))

	// The salvaged content must survive as text somewhere.
	if !strings.Contains(collected.String(), "stale output") {
		t.Errorf("orphaned tool result content should be salvaged as text: %q", collected.String())
	}
}