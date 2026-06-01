package connectors

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// encodeEventStreamFrame builds a minimal AWS EventStream frame with a single
// ":event-type" string header and a JSON payload. CRC fields are zeroed (the
// parser does not validate them). This mirrors the frame layout the Kiro
// connector decodes.
func encodeEventStreamFrame(eventType string, payload []byte) []byte {
	// Header: 1-byte name len, name, 1-byte type(7), 2-byte value len, value.
	name := ":event-type"
	var hdr bytes.Buffer
	hdr.WriteByte(byte(len(name)))
	hdr.WriteString(name)
	hdr.WriteByte(7) // string type
	var vl [2]byte
	binary.BigEndian.PutUint16(vl[:], uint16(len(eventType)))
	hdr.Write(vl[:])
	hdr.WriteString(eventType)

	headers := hdr.Bytes()
	headersLen := len(headers)
	totalLen := 12 + headersLen + len(payload) + 4 // prelude(8)+preludeCRC(4)+headers+payload+msgCRC(4)

	var out bytes.Buffer
	var u32 [4]byte
	binary.BigEndian.PutUint32(u32[:], uint32(totalLen))
	out.Write(u32[:])
	binary.BigEndian.PutUint32(u32[:], uint32(headersLen))
	out.Write(u32[:])
	out.Write([]byte{0, 0, 0, 0}) // prelude CRC (ignored)
	out.Write(headers)
	out.Write(payload)
	out.Write([]byte{0, 0, 0, 0}) // message CRC (ignored)
	return out.Bytes()
}

func TestEventStreamParser_DecodesFrames(t *testing.T) {
	var stream bytes.Buffer
	stream.Write(encodeEventStreamFrame("assistantResponseEvent", []byte(`{"content":"Hello"}`)))
	stream.Write(encodeEventStreamFrame("assistantResponseEvent", []byte(`{"content":" world"}`)))
	stream.Write(encodeEventStreamFrame("messageStopEvent", []byte(`{}`)))

	parser := newEventStreamParser(&stream)
	var events []string
	for {
		frame, err := parser.next()
		if err == errEventStreamEOF {
			break
		}
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if frame == nil {
			continue
		}
		events = append(events, frame.headers[":event-type"])
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 frames, got %d: %v", len(events), events)
	}
	if events[0] != "assistantResponseEvent" || events[2] != "messageStopEvent" {
		t.Errorf("unexpected event order: %v", events)
	}
}

func TestKiroFrameToChunks_TextAndStop(t *testing.T) {
	seen := map[string]bool{}
	hasTool := false

	frame := mustDecode(t, encodeEventStreamFrame("assistantResponseEvent", []byte(`{"content":"hi"}`)))
	chunks := kiroFrameToChunks(frame, seen, &hasTool)
	if len(chunks) != 1 || chunks[0].Type != core.ChunkText || chunks[0].Delta != "hi" {
		t.Fatalf("expected text chunk 'hi', got %+v", chunks)
	}

	stop := mustDecode(t, encodeEventStreamFrame("messageStopEvent", []byte(`{}`)))
	chunks = kiroFrameToChunks(stop, seen, &hasTool)
	if len(chunks) != 1 || chunks[0].Type != core.ChunkFinish || chunks[0].FinishReason != core.FinishStop {
		t.Fatalf("expected finish=stop, got %+v", chunks)
	}
}

func TestKiroFrameToChunks_ToolUse(t *testing.T) {
	seen := map[string]bool{}
	hasTool := false

	frame := mustDecode(t, encodeEventStreamFrame("toolUseEvent",
		[]byte(`{"toolUseId":"t1","name":"get_weather","input":{"city":"SF"}}`)))
	chunks := kiroFrameToChunks(frame, seen, &hasTool)
	if !hasTool {
		t.Error("hasTool should be set")
	}
	// First chunk announces the tool, second carries its arguments.
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d", len(chunks))
	}
	if chunks[0].ToolCall.Name != "get_weather" {
		t.Errorf("tool name wrong: %v", chunks[0].ToolCall.Name)
	}
	if !bytes.Contains(chunks[1].ToolCall.Arguments, []byte("SF")) {
		t.Errorf("tool args missing input: %s", chunks[1].ToolCall.Arguments)
	}
}

func mustDecode(t *testing.T, frame []byte) *eventStreamFrame {
	t.Helper()
	f, err := decodeEventStreamFrame(frame)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return f
}