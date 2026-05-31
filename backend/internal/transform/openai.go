package transform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// OpenAICodec handles the OpenAI Chat Completions wire format, the de-facto
// standard most CLI tools speak.
type OpenAICodec struct{}

func (OpenAICodec) Dialect() core.Dialect { return core.DialectOpenAI }

// ---- wire types -------------------------------------------------------------

type oaiRequest struct {
	Model       string        `json:"model"`
	Messages    []oaiMessage  `json:"messages"`
	Tools       []oaiTool     `json:"tools,omitempty"`
	ToolChoice  any           `json:"tool_choice,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	StreamOpts  *oaiStreamOpt `json:"stream_options,omitempty"`
}

type oaiStreamOpt struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type oaiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Parameters  json.RawMessage `json:"parameters,omitempty"`
	} `json:"function"`
}

// ---- request parsing --------------------------------------------------------

func (OpenAICodec) ParseRequest(body []byte) (*core.ChatRequest, error) {
	var raw oaiRequest
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("openai: parse request: %w", err)
	}

	req := &core.ChatRequest{
		Model:       raw.Model,
		Temperature: raw.Temperature,
		TopP:        raw.TopP,
		MaxTokens:   raw.MaxTokens,
		Stop:        raw.Stop,
		Stream:      raw.Stream,
		ToolChoice:  raw.ToolChoice,
	}

	for _, t := range raw.Tools {
		if t.Type != "function" && t.Type != "" {
			continue
		}
		req.Tools = append(req.Tools, core.Tool{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}

	for _, m := range raw.Messages {
		msg, isSystem, sysText := parseOAIMessage(m)
		if isSystem {
			// Hoist system content to the top-level field; concatenate multiples.
			if req.System != "" {
				req.System += "\n\n"
			}
			req.System += sysText
			continue
		}
		req.Messages = append(req.Messages, msg)
	}
	return req, nil
}

// parseOAIMessage converts one OpenAI message to canonical form. System and
// developer roles are reported separately so the caller can hoist them.
func parseOAIMessage(m oaiMessage) (msg core.Message, isSystem bool, sysText string) {
	role := m.Role
	// OpenAI's "developer" role is a system-equivalent for newer models.
	if role == "system" || role == "developer" {
		return core.Message{}, true, decodeOAIContentText(m.Content)
	}

	msg.Role = mapOAIRole(role)
	msg.Name = m.Name

	// Assistant tool calls.
	for _, tc := range m.ToolCalls {
		msg.Content = append(msg.Content, core.ContentPart{
			Type: core.PartToolCall,
			ToolCall: &core.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			},
		})
	}

	// Tool result message.
	if role == "tool" {
		msg.Content = append(msg.Content, core.ContentPart{
			Type: core.PartToolResult,
			ToolResult: &core.ToolResult{
				CallID:  m.ToolCallID,
				Content: decodeOAIContentText(m.Content),
			},
		})
		return msg, false, ""
	}

	// Text / multimodal content parts.
	msg.Content = append(msg.Content, decodeOAIContentParts(m.Content)...)
	return msg, false, ""
}

func mapOAIRole(role string) core.Role {
	switch role {
	case "user":
		return core.RoleUser
	case "assistant":
		return core.RoleAssistant
	case "tool":
		return core.RoleTool
	default:
		return core.RoleUser
	}
}

// decodeOAIContentText extracts plain text from an OpenAI content field, which
// may be a string or an array of typed parts.
func decodeOAIContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var b strings.Builder
	for _, p := range decodeOAIContentParts(raw) {
		if p.Type == core.PartText {
			b.WriteString(p.Text)
		}
	}
	return b.String()
}

// decodeOAIContentParts extracts canonical content parts from an OpenAI content
// field (string or array of {type,text|image_url}).
func decodeOAIContentParts(raw json.RawMessage) []core.ContentPart {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return nil
		}
		return []core.ContentPart{{Type: core.PartText, Text: s}}
	}

	var arr []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL struct {
			URL string `json:"url"`
		} `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}

	var parts []core.ContentPart
	for _, p := range arr {
		switch p.Type {
		case "text":
			parts = append(parts, core.ContentPart{Type: core.PartText, Text: p.Text})
		case "image_url":
			parts = append(parts, core.ContentPart{
				Type:  core.PartImage,
				Media: &core.MediaPayload{URL: p.ImageURL.URL},
			})
		}
	}
	return parts
}

// ---- request rendering ------------------------------------------------------

func (OpenAICodec) RenderRequest(req *core.ChatRequest) ([]byte, error) {
	out := oaiRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
		Stop:        req.Stop,
		Stream:      req.Stream,
		ToolChoice:  req.ToolChoice,
	}
	if req.Stream {
		out.StreamOpts = &oaiStreamOpt{IncludeUsage: true}
	}

	if req.System != "" {
		sysContent, _ := json.Marshal(req.System)
		out.Messages = append(out.Messages, oaiMessage{Role: "system", Content: sysContent})
	}

	for _, m := range req.Messages {
		out.Messages = append(out.Messages, renderOAIMessage(m))
	}

	for _, t := range req.Tools {
		var tool oaiTool
		tool.Type = "function"
		tool.Function.Name = t.Name
		tool.Function.Description = t.Description
		tool.Function.Parameters = t.Parameters
		out.Tools = append(out.Tools, tool)
	}

	return json.Marshal(out)
}

func renderOAIMessage(m core.Message) oaiMessage {
	out := oaiMessage{Role: string(m.Role), Name: m.Name}

	var textParts []string
	for _, p := range m.Content {
		switch p.Type {
		case core.PartText:
			textParts = append(textParts, p.Text)
		case core.PartToolCall:
			var tc oaiToolCall
			tc.ID = p.ToolCall.ID
			tc.Type = "function"
			tc.Function.Name = p.ToolCall.Name
			tc.Function.Arguments = string(p.ToolCall.Arguments)
			out.ToolCalls = append(out.ToolCalls, tc)
		case core.PartToolResult:
			out.Role = "tool"
			out.ToolCallID = p.ToolResult.CallID
			content, _ := json.Marshal(p.ToolResult.Content)
			out.Content = content
			return out
		}
	}

	if len(textParts) > 0 {
		content, _ := json.Marshal(strings.Join(textParts, ""))
		out.Content = content
	}
	return out
}

// ---- response parsing -------------------------------------------------------

type oaiResponse struct {
	ID      string `json:"id"`
	Model   string `json:"model"`
	Choices []struct {
		Message struct {
			Role      string        `json:"role"`
			Content   string        `json:"content"`
			ToolCalls []oaiToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *oaiUsage `json:"usage"`
}

type oaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (OpenAICodec) ParseResponse(body []byte, model string) (*core.ChatResponse, error) {
	var raw oaiResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("openai: parse response: %w", err)
	}
	if len(raw.Choices) == 0 {
		return nil, fmt.Errorf("openai: response has no choices")
	}

	choice := raw.Choices[0]
	msg := core.Message{Role: core.RoleAssistant}
	if choice.Message.Content != "" {
		msg.Content = append(msg.Content, core.ContentPart{Type: core.PartText, Text: choice.Message.Content})
	}
	for _, tc := range choice.Message.ToolCalls {
		msg.Content = append(msg.Content, core.ContentPart{
			Type: core.PartToolCall,
			ToolCall: &core.ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			},
		})
	}

	resp := &core.ChatResponse{
		ID:           raw.ID,
		Model:        firstNonEmpty(raw.Model, model),
		Message:      msg,
		FinishReason: mapOAIFinish(choice.FinishReason),
	}
	if raw.Usage != nil {
		resp.Usage = core.Usage{
			PromptTokens:     raw.Usage.PromptTokens,
			CompletionTokens: raw.Usage.CompletionTokens,
			TotalTokens:      raw.Usage.TotalTokens,
		}
	}
	return resp, nil
}

func (OpenAICodec) RenderResponse(resp *core.ChatResponse) ([]byte, error) {
	out := map[string]any{
		"id":      firstNonEmpty(resp.ID, "chatcmpl-"+resp.Model),
		"object":  "chat.completion",
		"model":   resp.Model,
		"choices": []map[string]any{renderOAIChoice(resp)},
		"usage": map[string]int{
			"prompt_tokens":     resp.Usage.PromptTokens,
			"completion_tokens": resp.Usage.CompletionTokens,
			"total_tokens":      resp.Usage.TotalTokens,
		},
	}
	return json.Marshal(out)
}

func renderOAIChoice(resp *core.ChatResponse) map[string]any {
	message := map[string]any{"role": "assistant"}
	var text strings.Builder
	var toolCalls []map[string]any
	for _, p := range resp.Message.Content {
		switch p.Type {
		case core.PartText:
			text.WriteString(p.Text)
		case core.PartToolCall:
			toolCalls = append(toolCalls, map[string]any{
				"id":   p.ToolCall.ID,
				"type": "function",
				"function": map[string]string{
					"name":      p.ToolCall.Name,
					"arguments": string(p.ToolCall.Arguments),
				},
			})
		}
	}
	if text.Len() > 0 {
		message["content"] = text.String()
	} else {
		message["content"] = nil
	}
	if len(toolCalls) > 0 {
		message["tool_calls"] = toolCalls
	}
	return map[string]any{
		"index":         0,
		"message":       message,
		"finish_reason": string(resp.FinishReason),
	}
}

func mapOAIFinish(r string) core.FinishReason {
	switch r {
	case "stop":
		return core.FinishStop
	case "length":
		return core.FinishLength
	case "tool_calls", "function_call":
		return core.FinishToolCalls
	case "content_filter":
		return core.FinishFilter
	default:
		return core.FinishStop
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
