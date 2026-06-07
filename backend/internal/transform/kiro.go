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

	// When the client did not send tools, flatten any tool calls/results in the
	// history into plain text. Kiro's validator requires a non-empty
	// currentMessage.userInputMessageContext.tools array whenever the history
	// references any structured tool use; omitting it returns "Improperly formed
	// request" (HTTP 400). Collapsing tool interactions to text keeps the request
	// honest and sidesteps that rule.
	messages := req.Messages
	if len(req.Tools) == 0 {
		messages = flattenKiroToolInteractions(messages)
	}

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

	for _, m := range messages {
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

	// Merge consecutive user messages (Kiro requires alternating roles). When
	// merging, also combine userInputMessageContext (toolResults/images) so the
	// second message's structured content is not silently dropped.
	var merged []map[string]any
	for _, h := range history {
		if uim, ok := h["userInputMessage"].(map[string]any); ok && len(merged) > 0 {
			if prev, ok := merged[len(merged)-1]["userInputMessage"].(map[string]any); ok {
				prev["content"] = prev["content"].(string) + "\n\n" + uim["content"].(string)
				mergeKiroUIMContext(prev, uim)
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

	// Reconcile orphaned toolResults — those whose toolUseId has no matching
	// toolUse in any assistant message. Client-side compaction can truncate the
	// assistant message containing the tool_use while keeping the tool_result;
	// the dangling structured reference makes Kiro return 400. Fold the orphaned
	// content back into the user text instead of discarding it. Only needed on
	// the tools-present path; flattenKiroToolInteractions already collapsed
	// everything to text when no tools were sent.
	if len(req.Tools) > 0 {
		reconcileOrphanedKiroToolResults(merged, current)
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

// kiroToolCallToText renders a tool call as a readable text line, used when
// collapsing structured tool interactions into plain text.
func kiroToolCallToText(name string, args json.RawMessage) string {
	argStr := "{}"
	if len(args) > 0 {
		argStr = string(args)
	}
	if strings.TrimSpace(name) == "" {
		name = "unknown"
	}
	return fmt.Sprintf("[Tool call: %s(%s)]", name, argStr)
}

// kiroToolResultToText renders a tool result as a readable text line.
func kiroToolResultToText(content string) string {
	return fmt.Sprintf("[Tool result: %s]", content)
}

// flattenKiroToolInteractions collapses every structured tool call/result in a
// conversation into plain text. Invoked only when the client did NOT send a
// tools array: without it, any structured tool reference in the history makes
// Kiro require currentMessage.userInputMessageContext.tools and otherwise
// return "Improperly formed request" (HTTP 400). Folding to text keeps the
// request honest and removes the structured content the validator keys on.
func flattenKiroToolInteractions(messages []core.Message) []core.Message {
	out := make([]core.Message, 0, len(messages))
	for _, m := range messages {
		switch m.Role {
		case core.RoleTool:
			var parts []string
			for _, p := range m.Content {
				if p.Type == core.PartToolResult && p.ToolResult != nil {
					parts = append(parts, kiroToolResultToText(p.ToolResult.Content))
				}
			}
			out = append(out, core.Message{
				Role:    core.RoleUser,
				Content: []core.ContentPart{{Type: core.PartText, Text: strings.Join(parts, "\n")}},
			})

		case core.RoleAssistant:
			var parts []string
			for _, p := range m.Content {
				switch p.Type {
				case core.PartText:
					if p.Text != "" {
						parts = append(parts, p.Text)
					}
				case core.PartToolCall:
					if p.ToolCall != nil {
						parts = append(parts, kiroToolCallToText(p.ToolCall.Name, p.ToolCall.Arguments))
					}
				}
			}
			out = append(out, core.Message{
				Role:    core.RoleAssistant,
				Content: []core.ContentPart{{Type: core.PartText, Text: strings.Join(parts, "\n")}},
			})

		default:
			// User/system: replace any tool_result parts with text, keep the rest
			// (text, images) untouched.
			newParts := make([]core.ContentPart, 0, len(m.Content))
			for _, p := range m.Content {
				if p.Type == core.PartToolResult && p.ToolResult != nil {
					newParts = append(newParts, core.ContentPart{
						Type: core.PartText,
						Text: kiroToolResultToText(p.ToolResult.Content),
					})
				} else {
					newParts = append(newParts, p)
				}
			}
			out = append(out, core.Message{Role: m.Role, Name: m.Name, Content: newParts})
		}
	}
	return out
}

// mergeKiroUIMContext merges the userInputMessageContext of src into dst when
// two consecutive user messages are combined, so toolResults from the second
// message survive the merge.
func mergeKiroUIMContext(dst, src map[string]any) {
	srcCtx, ok := src["userInputMessageContext"].(map[string]any)
	if !ok || len(srcCtx) == 0 {
		return
	}
	dstCtx, ok := dst["userInputMessageContext"].(map[string]any)
	if !ok {
		dst["userInputMessageContext"] = srcCtx
		return
	}
	if srcTR, ok := srcCtx["toolResults"].([]map[string]any); ok && len(srcTR) > 0 {
		dstTR, _ := dstCtx["toolResults"].([]map[string]any)
		dstCtx["toolResults"] = append(dstTR, srcTR...)
	}
	if srcImg, ok := srcCtx["images"].([]map[string]any); ok && len(srcImg) > 0 {
		dstImg, _ := dstCtx["images"].([]map[string]any)
		dstCtx["images"] = append(dstImg, srcImg...)
	}
}

// reconcileOrphanedKiroToolResults removes toolResults whose toolUseId has no
// matching toolUse in any assistant message, folding their content back into
// the carrier's user text. A dangling structured reference makes Kiro return
// 400, but the client deliberately kept the result through compaction, so the
// content is salvaged as text rather than discarded.
// userInputMessage map; orphans can land on it or on any history user turn.
func reconcileOrphanedKiroToolResults(history []map[string]any, current map[string]any) {
	// Phase 1: collect valid toolUseIds from assistant history.
	valid := map[string]bool{}
	for _, h := range history {
		arm, ok := h["assistantResponseMessage"].(map[string]any)
		if !ok {
			continue
		}
		tus, ok := arm["toolUses"].([]map[string]any)
		if !ok {
			continue
		}
		for _, tu := range tus {
			if id, ok := tu["toolUseId"].(string); ok && id != "" {
				valid[id] = true
			}
		}
	}

	// Phase 2: across history + current, keep results with a matching toolUse
	// and salvage the rest as text.
	for _, h := range history {
		if uim, ok := h["userInputMessage"].(map[string]any); ok {
			reconcileKiroUIMOrphans(uim, valid)
		}
	}
	if current != nil {
		reconcileKiroUIMOrphans(current, valid)
	}
}

// reconcileKiroUIMOrphans salvages orphaned toolResults on a single
// userInputMessage map.
func reconcileKiroUIMOrphans(uim map[string]any, valid map[string]bool) {
	ctx, ok := uim["userInputMessageContext"].(map[string]any)
	if !ok {
		return
	}
	trs, ok := ctx["toolResults"].([]map[string]any)
	if !ok || len(trs) == 0 {
		return
	}

	var kept []map[string]any
	var salvaged []string
	for _, tr := range trs {
		id, _ := tr["toolUseId"].(string)
		if valid[id] {
			kept = append(kept, tr)
		} else {
			salvaged = append(salvaged, kiroToolResultToText(extractKiroToolResultText(tr)))
		}
	}

	if len(salvaged) == 0 {
		return // no orphans — leave untouched
	}

	extra := strings.Join(salvaged, "\n")
	if cur, _ := uim["content"].(string); cur != "" {
		uim["content"] = cur + "\n\n" + extra
	} else {
		uim["content"] = extra
	}

	if len(kept) > 0 {
		ctx["toolResults"] = kept
	} else {
		delete(ctx, "toolResults")
	}
	if len(ctx) == 0 {
		delete(uim, "userInputMessageContext")
	}
}

// extractKiroToolResultText pulls the concatenated text out of a Kiro
// toolResult's content array ([]{"text": ...}).
func extractKiroToolResultText(tr map[string]any) string {
	content, ok := tr["content"].([]map[string]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, c := range content {
		if t, ok := c["text"].(string); ok {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n")
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
