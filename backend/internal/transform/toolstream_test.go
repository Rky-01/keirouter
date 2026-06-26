package transform

import (
	"bytes"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// TestAnthropicRenderRepeatedToolID reproduces the Kiro streaming pattern that
// broke Cline: the connector emits the tool ID on both the open chunk and the
// arguments-continuation chunk. The renderer must treat the repeated ID as a
// continuation of the same tool_use block, not open a second nameless block.
func TestAnthropicRenderRepeatedToolID(t *testing.T) {
	codec := AnthropicCodec{}
	state := &StreamState{Model: "claude"}

	open := core.StreamChunk{
		Type: core.ChunkToolCall,
		ToolCall: &core.ToolCall{
			ID:        "tool_1",
			Name:      "ask_followup_question",
			Arguments: nil,
		},
	}
	args := core.StreamChunk{
		Type: core.ChunkToolCall,
		ToolCall: &core.ToolCall{
			ID:        "tool_1",
			Arguments: []byte(`{"question":"Which file?"}`),
		},
	}

	var all []byte
	for _, ch := range []core.StreamChunk{open, args} {
		events, err := codec.RenderStreamChunk(ch, state)
		if err != nil {
			t.Fatalf("RenderStreamChunk: %v", err)
		}
		for _, ev := range events {
			all = append(all, ev...)
		}
	}

	if got := bytes.Count(all, []byte(`"type":"tool_use"`)); got != 1 {
		t.Fatalf("expected exactly 1 tool_use block, got %d\n%s", got, all)
	}
	if !bytes.Contains(all, []byte(`"name":"ask_followup_question"`)) {
		t.Fatalf("expected tool_use block to carry the tool name\n%s", all)
	}
	if bytes.Contains(all, []byte(`"name":""`)) {
		t.Fatalf("a nameless duplicate tool_use block was opened\n%s", all)
	}
	if !bytes.Contains(all, []byte("input_json_delta")) || !bytes.Contains(all, []byte("Which file?")) {
		t.Fatalf("expected arguments to stream as input_json_delta\n%s", all)
	}
}
