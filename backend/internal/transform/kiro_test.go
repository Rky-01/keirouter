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