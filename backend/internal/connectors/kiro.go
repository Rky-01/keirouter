package connectors

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"

	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/core"
	"github.com/mydisha/keirouter/backend/internal/transform"
)

// Kiro drives AWS CodeWhisperer's generateAssistantResponse endpoint used by
// Kiro AI. The request is a conversationState payload (built by the Kiro codec);
// the response is a binary AWS EventStream of typed events
// (assistantResponseEvent, reasoningContentEvent, toolUseEvent, messageStopEvent,
// metricsEvent, ...). This connector parses the binary frames and maps them to
// canonical chunks, mirroring 9router's KiroExecutor.transformEventStreamToSSE.
type Kiro struct {
	id          string
	defaultBase string
	codec       transform.KiroCodec
}

// NewKiro builds a Kiro connector.
func NewKiro(id, defaultBaseURL string) *Kiro {
	return &Kiro{id: id, defaultBase: defaultBaseURL}
}

func (c *Kiro) ID() string            { return c.id }
func (c *Kiro) Dialect() core.Dialect { return core.DialectKiro }

func (c *Kiro) baseURL(creds core.Credentials) string {
	if creds.BaseURL != "" {
		return creds.BaseURL
	}
	return c.defaultBase
}

// headers builds the AWS SDK + CodeWhisperer headers Kiro expects.
func (c *Kiro) headers(creds core.Credentials) map[string]string {
	h := map[string]string{
		"Accept":                 "application/vnd.amazon.eventstream",
		"X-Amz-Target":           "AmazonCodeWhispererStreamingService.GenerateAssistantResponse",
		"User-Agent":             "AWS-SDK-JS/3.0.0 kiro-ide/1.0.0",
		"X-Amz-User-Agent":       "aws-sdk-js/3.0.0 kiro-ide/1.0.0",
		"Amz-Sdk-Request":        "attempt=1; max=3",
		"Amz-Sdk-Invocation-Id":  uuid.NewString(),
	}
	if creds.AccessToken != "" {
		h["Authorization"] = bearer(creds.AccessToken)
	}
	return mergeHeaders(h, creds.Headers)
}

// Chat performs a non-streaming call by draining the event stream and folding
// the chunks into a single response.
func (c *Kiro) Chat(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (*core.ChatResponse, error) {
	stream, err := c.Stream(ctx, req, creds)
	if err != nil {
		return nil, err
	}

	msg := core.Message{Role: core.RoleAssistant}
	var text, thinking string
	toolCalls := map[string]*core.ToolCall{}
	var toolOrder []string
	finish := core.FinishStop
	var usage core.Usage

	for ch := range stream {
		switch ch.Type {
		case core.ChunkText:
			text += ch.Delta
		case core.ChunkThinking:
			thinking += ch.Delta
		case core.ChunkToolCall:
			if ch.ToolCall != nil {
				existing, ok := toolCalls[ch.ToolCall.ID]
				if !ok {
					tc := *ch.ToolCall
					toolCalls[ch.ToolCall.ID] = &tc
					toolOrder = append(toolOrder, ch.ToolCall.ID)
				} else if len(ch.ToolCall.Arguments) > 0 {
					existing.Arguments = append(existing.Arguments, ch.ToolCall.Arguments...)
				}
				finish = core.FinishToolCalls
			}
		case core.ChunkFinish:
			if ch.FinishReason != "" {
				finish = ch.FinishReason
			}
		case core.ChunkUsage:
			if ch.Usage != nil {
				usage = *ch.Usage
			}
		case core.ChunkError:
			if ch.Err != nil {
				return nil, ch.Err
			}
		}
	}

	if thinking != "" {
		msg.Content = append(msg.Content, core.ContentPart{Type: core.PartThinking, Text: thinking})
	}
	if text != "" {
		msg.Content = append(msg.Content, core.ContentPart{Type: core.PartText, Text: text})
	}
	for _, id := range toolOrder {
		tc := toolCalls[id]
		if len(tc.Arguments) == 0 {
			tc.Arguments = json.RawMessage("{}")
		}
		msg.Content = append(msg.Content, core.ContentPart{Type: core.PartToolCall, ToolCall: tc})
	}

	return &core.ChatResponse{Model: req.Model, Message: msg, FinishReason: finish, Usage: usage}, nil
}

// Stream performs a streaming call, parsing the AWS EventStream into canonical
// chunks.
func (c *Kiro) Stream(ctx context.Context, req *core.ChatRequest, creds core.Credentials) (<-chan core.StreamChunk, error) {
	body, err := c.codec.RenderRequest(req)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err}
	}

	// Kiro returns a binary eventstream, not SSE; use a plain streaming POST.
	resp, err := openStream(ctx, c.id, req.Model, c.baseURL(creds), body, c.headers(creds))
	if err != nil {
		return nil, err
	}

	out := make(chan core.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		parser := newEventStreamParser(resp.Body)
		seenTools := map[string]bool{}
		hasTool := false

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			frame, err := parser.next()
			if err != nil {
				if err != errEventStreamEOF {
					out <- core.StreamChunk{
						Type: core.ChunkError,
						Err:  &core.ProviderError{Kind: core.ErrUpstream, Provider: c.id, Model: req.Model, Message: err.Error(), Cause: err},
					}
				}
				break
			}
			if frame == nil {
				continue
			}

			for _, ch := range kiroFrameToChunks(frame, seenTools, &hasTool) {
				select {
				case out <- ch:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

// kiroFrameToChunks maps one decoded Kiro eventstream frame to canonical chunks.
func kiroFrameToChunks(frame *eventStreamFrame, seenTools map[string]bool, hasTool *bool) []core.StreamChunk {
	eventType := frame.headers[":event-type"]
	var chunks []core.StreamChunk

	switch eventType {
	case "assistantResponseEvent", "codeEvent":
		var p struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(frame.payload, &p) == nil && p.Content != "" {
			chunks = append(chunks, core.StreamChunk{Type: core.ChunkText, Delta: p.Content})
		}

	case "reasoningContentEvent":
		text := extractKiroReasoning(frame.payload)
		if text != "" {
			chunks = append(chunks, core.StreamChunk{Type: core.ChunkThinking, Delta: text})
		}

	case "toolUseEvent":
		chunks = append(chunks, kiroToolUseChunks(frame.payload, seenTools)...)
		if len(chunks) > 0 {
			*hasTool = true
		}

	case "messageStopEvent":
		reason := core.FinishStop
		if *hasTool {
			reason = core.FinishToolCalls
		}
		chunks = append(chunks, core.StreamChunk{Type: core.ChunkFinish, FinishReason: reason})

	case "metricsEvent":
		var p struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
		}
		// metrics may be nested under metricsEvent.
		raw := frame.payload
		var wrap map[string]json.RawMessage
		if json.Unmarshal(frame.payload, &wrap) == nil {
			if m, ok := wrap["metricsEvent"]; ok {
				raw = m
			}
		}
		if json.Unmarshal(raw, &p) == nil && (p.InputTokens > 0 || p.OutputTokens > 0) {
			chunks = append(chunks, core.StreamChunk{
				Type: core.ChunkUsage,
				Usage: &core.Usage{
					PromptTokens:     p.InputTokens,
					CompletionTokens: p.OutputTokens,
					TotalTokens:      p.InputTokens + p.OutputTokens,
				},
			})
		}
	}
	return chunks
}

func extractKiroReasoning(payload []byte) string {
	// Payload may be a string, {text|content}, or {reasoningContentEvent:{...}}.
	var asString string
	if json.Unmarshal(payload, &asString) == nil && asString != "" {
		return asString
	}
	var obj struct {
		Text                  string `json:"text"`
		Content               string `json:"content"`
		ReasoningContentEvent struct {
			Text    string `json:"text"`
			Content string `json:"content"`
		} `json:"reasoningContentEvent"`
	}
	if json.Unmarshal(payload, &obj) != nil {
		return ""
	}
	if obj.Text != "" {
		return obj.Text
	}
	if obj.Content != "" {
		return obj.Content
	}
	if obj.ReasoningContentEvent.Text != "" {
		return obj.ReasoningContentEvent.Text
	}
	return obj.ReasoningContentEvent.Content
}

func kiroToolUseChunks(payload []byte, seenTools map[string]bool) []core.StreamChunk {
	parseOne := func(raw json.RawMessage) []core.StreamChunk {
		var t struct {
			ToolUseID string          `json:"toolUseId"`
			Name      string          `json:"name"`
			Input     json.RawMessage `json:"input"`
		}
		if json.Unmarshal(raw, &t) != nil {
			return nil
		}
		id := t.ToolUseID
		if id == "" {
			id = "call_" + uuid.NewString()
		}
		var chunks []core.StreamChunk
		if !seenTools[id] {
			seenTools[id] = true
			chunks = append(chunks, core.StreamChunk{
				Type:     core.ChunkToolCall,
				ToolCall: &core.ToolCall{ID: id, Name: t.Name, Arguments: json.RawMessage("")},
			})
		}
		if len(t.Input) > 0 {
			args := t.Input
			// Input may be a JSON string or object; normalize to a string of JSON.
			var asStr string
			if json.Unmarshal(t.Input, &asStr) == nil {
				args = json.RawMessage(asStr)
			}
			chunks = append(chunks, core.StreamChunk{
				Type:     core.ChunkToolCall,
				ToolCall: &core.ToolCall{ID: id, Arguments: args},
			})
		}
		return chunks
	}

	// Payload may be a single tool-use object or an array.
	trimmed := payload
	for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\n' || trimmed[0] == '\t') {
		trimmed = trimmed[1:]
	}
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var arr []json.RawMessage
		if json.Unmarshal(payload, &arr) != nil {
			return nil
		}
		var chunks []core.StreamChunk
		for _, item := range arr {
			chunks = append(chunks, parseOne(item)...)
		}
		return chunks
	}
	return parseOne(payload)
}

// ---- AWS EventStream binary parser ------------------------------------------

// errEventStreamEOF signals the stream ended cleanly.
var errEventStreamEOF = errEventStreamDone{}

type errEventStreamDone struct{}

func (errEventStreamDone) Error() string { return "eventstream: EOF" }

// eventStreamFrame is one decoded AWS EventStream message: its string headers
// plus the raw JSON payload.
type eventStreamFrame struct {
	headers map[string]string
	payload []byte
}

// eventStreamParser reads AWS EventStream binary frames from an io.Reader. Each
// frame is: [4-byte total length][4-byte headers length][4-byte prelude CRC]
// [headers][payload][4-byte message CRC], all big-endian. Headers are
// length-prefixed name/value pairs; only string headers (type 7) are decoded,
// which is all CodeWhisperer emits (:event-type, :content-type, :message-type).
type eventStreamParser struct {
	r   io.Reader
	buf []byte
}

func newEventStreamParser(r io.Reader) *eventStreamParser {
	return &eventStreamParser{r: r}
}

// next returns the next decoded frame, errEventStreamEOF at end of stream, or a
// transport error. It returns (nil, nil) for a frame that decoded to no usable
// content so the caller can continue.
func (p *eventStreamParser) next() (*eventStreamFrame, error) {
	// Ensure at least the 12-byte prelude is buffered.
	for len(p.buf) < 12 {
		if err := p.fill(); err != nil {
			if err == io.EOF && len(p.buf) == 0 {
				return nil, errEventStreamEOF
			}
			if err == io.EOF {
				return nil, errEventStreamEOF
			}
			return nil, err
		}
	}

	totalLen := int(binary.BigEndian.Uint32(p.buf[0:4]))
	if totalLen < 16 {
		return nil, fmt.Errorf("eventstream: invalid frame length %d", totalLen)
	}

	// Buffer the whole frame.
	for len(p.buf) < totalLen {
		if err := p.fill(); err != nil {
			if err == io.EOF {
				return nil, errEventStreamEOF
			}
			return nil, err
		}
	}

	frame := p.buf[:totalLen]
	p.buf = p.buf[totalLen:]

	return decodeEventStreamFrame(frame)
}

// fill reads more bytes into the buffer.
func (p *eventStreamParser) fill() error {
	tmp := make([]byte, 32*1024)
	n, err := p.r.Read(tmp)
	if n > 0 {
		p.buf = append(p.buf, tmp[:n]...)
	}
	return err
}

// decodeEventStreamFrame parses one complete frame's bytes.
func decodeEventStreamFrame(frame []byte) (*eventStreamFrame, error) {
	if len(frame) < 16 {
		return nil, fmt.Errorf("eventstream: short frame")
	}
	headersLen := int(binary.BigEndian.Uint32(frame[4:8]))

	headers := map[string]string{}
	offset := 12 // after prelude (8) + prelude CRC (4)
	headerEnd := 12 + headersLen
	if headerEnd > len(frame) {
		return nil, fmt.Errorf("eventstream: headers exceed frame")
	}

	for offset < headerEnd {
		if offset >= len(frame) {
			break
		}
		nameLen := int(frame[offset])
		offset++
		if offset+nameLen > len(frame) {
			break
		}
		name := string(frame[offset : offset+nameLen])
		offset += nameLen
		if offset >= len(frame) {
			break
		}
		headerType := frame[offset]
		offset++
		if headerType == 7 { // string
			if offset+2 > len(frame) {
				break
			}
			valueLen := int(binary.BigEndian.Uint16(frame[offset : offset+2]))
			offset += 2
			if offset+valueLen > len(frame) {
				break
			}
			headers[name] = string(frame[offset : offset+valueLen])
			offset += valueLen
		} else {
			// Non-string header: we can't size it reliably, stop parsing headers.
			break
		}
	}

	payloadStart := 12 + headersLen
	payloadEnd := len(frame) - 4 // exclude message CRC
	if payloadEnd < payloadStart {
		return &eventStreamFrame{headers: headers, payload: nil}, nil
	}
	payload := frame[payloadStart:payloadEnd]
	return &eventStreamFrame{headers: headers, payload: payload}, nil
}
