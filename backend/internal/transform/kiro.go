package transform

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// KiroCodec renders canonical requests to AWS CodeWhisperer's
// generateAssistantResponse format used by Kiro. The wire shape is a
// conversationState envelope: a currentMessage.userInputMessage plus a history
// of alternating user/assistant turns. System and tool turns fold into user
// turns; consecutive same-role turns merge. Tools attach to the current
// message as toolSpecification entries; tool results as toolResults.
//
// Kiro has no native reasoning toggle, so reasoning is enabled by injecting a
// "<thinking_mode>enabled</thinking_mode>" prefix into the user content, plus a
// "[Context: Current time ...]" marker — a faithful port of 9router's
// openai-to-kiro translator and kiroConstants. The response is a binary AWS
// EventStream, parsed by the Kiro connector (not this codec), so the
// Parse/RenderResponse and stream methods here are minimal stubs.
type KiroCodec struct{}

func (KiroCodec) Dialect() core.Dialect { return core.DialectKiro }

const (
	kiroThinkingBudgetDefault = 16000
	kiroAgenticSuffix         = "-agentic"
	kiroThinkingSuffix        = "-thinking"
)

// kiroAgenticSystemPrompt mirrors KIRO_AGENTIC_SYSTEM_PROMPT (chunked-write
// protocol) injected for synthetic "-agentic" model variants.
const kiroAgenticSystemPrompt = `# CRITICAL: CHUNKED WRITE PROTOCOL (MANDATORY)

You MUST follow these rules for ALL file operations. Violation causes server timeouts and task failure.

## ABSOLUTE LIMITS
- **MAXIMUM 350 LINES** per single write/edit operation - NO EXCEPTIONS
- **RECOMMENDED 300 LINES** or less for optimal performance
- **NEVER** write entire files in one operation if >300 lines

## MANDATORY CHUNKED WRITE STRATEGY
For new files >300 lines, write an initial 250-300 line chunk then append in
250-300 line chunks. For edits, use surgical/targeted edits only. For large code
generation, emit logical sections as separate operations.

REMEMBER: When in doubt, write LESS per operation. Multiple small operations > one large operation.`

// buildThinkingSystemPrefix mirrors kiroConstants.buildThinkingSystemPrefix.
func buildKiroThinkingPrefix(budget int) string {
	if budget < 1 {
		budget = kiroThinkingBudgetDefault
	}
	if budget > 32000 {
		budget = 32000
	}
	return fmt.Sprintf("<thinking_mode>enabled</thinking_mode>\n<max_thinking_length>%d</max_thinking_length>", budget)
}

// resolveKiroModel strips synthetic suffixes and reports the implied behaviours.
func resolveKiroModel(model string) (upstream string, agentic, thinking bool) {
	upstream = model
	if strings.HasSuffix(upstream, kiroAgenticSuffix) {
		agentic = true
		upstream = strings.TrimSuffix(upstream, kiroAgenticSuffix)
	}
	if strings.HasSuffix(upstream, kiroThinkingSuffix) {
		thinking = true
		upstream = strings.TrimSuffix(upstream, kiroThinkingSuffix)
	}
	return upstream, agentic, thinking
}

// kiroThinkingEnabled detects reasoning intent from the canonical request,
// mirroring kiroConstants.isThinkingEnabled (model hint, reasoning config,
// explicit thinking, or a <thinking_mode> tag in the prompt text).
func kiroThinkingEnabled(req *core.ChatRequest, model string) bool {
	if req.Reasoning != nil {
		switch strings.ToLower(req.Reasoning.Effort) {
		case "low", "medium", "high", "auto":
			return true
		}
	}
	m := strings.ToLower(model)
	if strings.Contains(m, "thinking") || strings.Contains(m, "-reason") {
		return true
	}
	if strings.Contains(req.System, "<thinking_mode>enabled</thinking_mode>") ||
		strings.Contains(req.System, "<thinking_mode>interleaved</thinking_mode>") {
		return true
	}
	for _, msg := range req.Messages {
		if msg.Role != core.RoleUser && msg.Role != core.RoleSystem {
			continue
		}
		if strings.Contains(msg.TextContent(), "<thinking_mode>enabled</thinking_mode>") {
			return true
		}
	}
	return false
}

func (KiroCodec) ParseRequest(body []byte) (*core.ChatRequest, error) {
	// Kiro is upstream-only; a minimal decode is enough.
	var env struct {
		ConversationState struct {
			CurrentMessage struct {
				UserInputMessage struct {
					Content string `json:"content"`
					ModelID string `json:"modelId"`
				} `json:"userInputMessage"`
			} `json:"currentMessage"`
		} `json:"conversationState"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("kiro: parse request: %w", err)
	}
	cm := env.ConversationState.CurrentMessage.UserInputMessage
	return &core.ChatRequest{
		Model:    cm.ModelID,
		Messages: []core.Message{{Role: core.RoleUser, Content: []core.ContentPart{{Type: core.PartText, Text: cm.Content}}}},
	}, nil
}

// RenderRequest builds the CodeWhisperer conversationState payload.
func (KiroCodec) RenderRequest(req *core.ChatRequest) ([]byte, error) {
	upstream, agentic, modelThinking := resolveKiroModel(req.Model)
	thinking := modelThinking || kiroThinkingEnabled(req, req.Model)

	history, current := buildKiroHistory(req, upstream)

	// Compose the final user content with the system prefix.
	finalContent, _ := current["content"].(string)
	var prefix []string
	if thinking {
		prefix = append(prefix, buildKiroThinkingPrefix(kiroThinkingBudgetDefault))
	}
	prefix = append(prefix, "[Context: Current time is "+time.Now().UTC().Format(time.RFC3339)+"]")
	if agentic {
		prefix = append(prefix, kiroAgenticSystemPrompt)
	}
	// Prepend the system prompt (Kiro folds system into the user content).
	if req.System != "" {
		prefix = append(prefix, req.System)
	}
	current["content"] = strings.Join(prefix, "\n\n") + "\n\n" + finalContent
	current["modelId"] = upstream
	current["origin"] = "AI_EDITOR"

	payload := map[string]any{
		"conversationState": map[string]any{
			"chatTriggerType": "MANUAL",
			"conversationId":  uuid.NewString(),
			"currentMessage":  map[string]any{"userInputMessage": current},
			"history":         history,
		},
	}

	infer := map[string]any{"maxTokens": 32000}
	if req.Temperature != nil {
		infer["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		infer["topP"] = *req.TopP
	}
	payload["inferenceConfig"] = infer

	return json.Marshal(payload)
}

// buildKiroHistory converts canonical messages into Kiro history + the current
// user message map. System/tool roles fold into user; consecutive user turns
// merge; tools attach to the current message.
func buildKiroHistory(req *core.ChatRequest, upstream string) ([]map[string]any, map[string]any) {
	var history []map[string]any

	flushUser := func(text string, toolResults []map[string]any, images []map[string]any) {
		uim := map[string]any{"content": firstNonEmptyStr(text, "continue"), "modelId": upstream}
		if len(images) > 0 {
			uim["images"] = images
		}
		ctx := map[string]any{}
		if len(toolResults) > 0 {
			ctx["toolResults"] = toolResults
		}
		if len(ctx) > 0 {
			uim["userInputMessageContext"] = ctx
		}
		history = append(history, map[string]any{"userInputMessage": uim})
	}

	for _, m := range req.Messages {
		switch m.Role {
		case core.RoleAssistant:
			content := strings.TrimSpace(m.TextContent())
			if content == "" {
				content = "..."
			}
			arm := map[string]any{"content": content}
			var toolUses []map[string]any
			for _, p := range m.Content {
				if p.Type == core.PartToolCall && p.ToolCall != nil {
					toolUses = append(toolUses, map[string]any{
						"toolUseId": firstNonEmptyStr(p.ToolCall.ID, uuid.NewString()),
						"name":      p.ToolCall.Name,
						"input":     rawToAny(p.ToolCall.Arguments),
					})
				}
			}
			if len(toolUses) > 0 {
				arm["toolUses"] = toolUses
			}
			history = append(history, map[string]any{"assistantResponseMessage": arm})

		case core.RoleTool:
			var toolResults []map[string]any
			for _, p := range m.Content {
				if p.Type == core.PartToolResult && p.ToolResult != nil {
					toolResults = append(toolResults, map[string]any{
						"toolUseId": p.ToolResult.CallID,
						"status":    "success",
						"content":   []map[string]any{{"text": p.ToolResult.Content}},
					})
				}
			}
			flushUser("", toolResults, nil)

		default: // user, system
			var text strings.Builder
			var images []map[string]any
			for _, p := range m.Content {
				switch p.Type {
				case core.PartText:
					if text.Len() > 0 {
						text.WriteString("\n")
					}
					text.WriteString(p.Text)
				case core.PartImage:
					if p.Media != nil && p.Media.Data != "" {
						format := mimeToFormat(p.Media.MIMEType)
						images = append(images, map[string]any{
							"format": format,
							"source": map[string]any{"bytes": p.Media.Data},
						})
					}
				}
			}
			flushUser(text.String(), nil, images)
		}
	}

	// Merge consecutive user messages (Kiro requires alternating roles).
	var merged []map[string]any
	for _, h := range history {
		if uim, ok := h["userInputMessage"].(map[string]any); ok && len(merged) > 0 {
			if prev, ok := merged[len(merged)-1]["userInputMessage"].(map[string]any); ok {
				prev["content"] = prev["content"].(string) + "\n\n" + uim["content"].(string)
				continue
			}
		}
		merged = append(merged, h)
	}

	// Pop the last user message as the current message.
	current := map[string]any{"content": ""}
	for i := len(merged) - 1; i >= 0; i-- {
		if uim, ok := merged[i]["userInputMessage"].(map[string]any); ok {
			current = uim
			merged = append(merged[:i], merged[i+1:]...)
			break
		}
	}

	// Attach tools to the current message's context.
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, t := range req.Tools {
			schema := rawToAny(t.Parameters)
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}, "required": []any{}}
			}
			desc := t.Description
			if strings.TrimSpace(desc) == "" {
				desc = "Tool: " + t.Name
			}
			tools = append(tools, map[string]any{
				"toolSpecification": map[string]any{
					"name":        t.Name,
					"description": desc,
					"inputSchema": map[string]any{"json": schema},
				},
			})
		}
		ctx, _ := current["userInputMessageContext"].(map[string]any)
		if ctx == nil {
			ctx = map[string]any{}
		}
		ctx["tools"] = tools
		current["userInputMessageContext"] = ctx
	}

	return merged, current
}

// ParseResponse / RenderResponse / stream methods: Kiro responses are binary
// AWS EventStream, parsed by the connector. These satisfy the Codec interface.
func (KiroCodec) ParseResponse(_ []byte, model string) (*core.ChatResponse, error) {
	return &core.ChatResponse{Model: model, Message: core.Message{Role: core.RoleAssistant}}, nil
}

func (KiroCodec) RenderResponse(resp *core.ChatResponse) ([]byte, error) {
	return json.Marshal(map[string]any{"model": resp.Model})
}

func mimeToFormat(mime string) string {
	if i := strings.Index(mime, "/"); i >= 0 && i+1 < len(mime) {
		return mime[i+1:]
	}
	if mime == "" {
		return "png"
	}
	return mime
}

func rawToAny(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return v
}

func firstNonEmptyStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}