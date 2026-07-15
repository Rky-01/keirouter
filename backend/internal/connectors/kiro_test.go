package connectors

import (
	"bytes"
	"context"
	"encoding/binary"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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

func TestKiroFrameToChunks_UsageEvent(t *testing.T) {
	seen := map[string]bool{}
	hasTool := false

	// Top-level inputTokens/outputTokens under usageEvent name.
	frame := mustDecode(t, encodeEventStreamFrame("usageEvent",
		[]byte(`{"inputTokens":120,"outputTokens":34}`)))
	chunks := kiroFrameToChunks(frame, seen, &hasTool)
	if len(chunks) != 1 || chunks[0].Type != core.ChunkUsage || chunks[0].Usage == nil {
		t.Fatalf("expected usage chunk, got %+v", chunks)
	}
	if chunks[0].Usage.PromptTokens != 120 || chunks[0].Usage.CompletionTokens != 34 {
		t.Errorf("usage tokens wrong: %+v", chunks[0].Usage)
	}
	if chunks[0].Usage.TotalTokens != 154 {
		t.Errorf("total tokens wrong: %d", chunks[0].Usage.TotalTokens)
	}
}

func TestKiroFrameToChunks_MetricsEventNested(t *testing.T) {
	seen := map[string]bool{}
	hasTool := false

	// Counts nested under a key matching the event type.
	frame := mustDecode(t, encodeEventStreamFrame("metricsEvent",
		[]byte(`{"metricsEvent":{"inputTokens":10,"outputTokens":5}}`)))
	chunks := kiroFrameToChunks(frame, seen, &hasTool)
	if len(chunks) != 1 || chunks[0].Type != core.ChunkUsage {
		t.Fatalf("expected usage chunk, got %+v", chunks)
	}
	if chunks[0].Usage.PromptTokens != 10 || chunks[0].Usage.CompletionTokens != 5 {
		t.Errorf("nested usage wrong: %+v", chunks[0].Usage)
	}
}

func TestParseKiroUsage_ZeroReturnsNil(t *testing.T) {
	if u := parseKiroUsage("usageEvent", []byte(`{"inputTokens":0,"outputTokens":0}`)); u != nil {
		t.Errorf("expected nil for zero usage, got %+v", u)
	}
	if u := parseKiroUsage("usageEvent", []byte(`not json`)); u != nil {
		t.Errorf("expected nil for bad json, got %+v", u)
	}
}

func TestEstimateKiroUsage(t *testing.T) {
	req := &core.ChatRequest{
		System: "you are helpful", // 15 chars
		Messages: []core.Message{
			{Role: core.RoleUser, Content: []core.ContentPart{
				{Type: core.PartText, Text: "hello there"}, // 11 chars
			}},
		},
	}
	// input chars = 15 + 11 = 26 → ceil(26/4) = 7 prompt tokens
	// output chars = 40 → ceil(40/4) = 10 completion tokens
	u := estimateKiroUsage(req, 40)
	if u == nil {
		t.Fatal("expected non-nil estimate")
	}
	if u.PromptTokens != 7 {
		t.Errorf("prompt tokens = %d, want 7", u.PromptTokens)
	}
	if u.CompletionTokens != 10 {
		t.Errorf("completion tokens = %d, want 10", u.CompletionTokens)
	}
	if u.TotalTokens != 17 {
		t.Errorf("total tokens = %d, want 17", u.TotalTokens)
	}

	// Empty request + zero output → nil (nothing to record).
	if got := estimateKiroUsage(&core.ChatRequest{}, 0); got != nil {
		t.Errorf("expected nil for empty request, got %+v", got)
	}
}

func TestCharsToTokens(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, 0}, {-5, 0}, {1, 1}, {4, 1}, {5, 2}, {8, 2}, {9, 3},
	}
	for _, c := range cases {
		if got := charsToTokens(c.in); got != c.want {
			t.Errorf("charsToTokens(%d) = %d, want %d", c.in, got, c.want)
		}
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

func TestKiroEndpoints_OAuthDefaultOrder(t *testing.T) {
	c := NewKiro("kiro", kiroEndpoints[0])
	got := c.endpoints(core.Credentials{})
	if len(got) != len(kiroEndpoints) {
		t.Fatalf("expected %d endpoints, got %d", len(kiroEndpoints), len(got))
	}
	// OAuth keeps the default order: kiro.dev leads.
	if !strings.Contains(got[0], "kiro.dev") {
		t.Errorf("oauth should lead with kiro.dev, got %s", got[0])
	}
}

func TestKiroEndpoints_APIKeyPrefersAmazon(t *testing.T) {
	c := NewKiro("kiro", kiroEndpoints[0])
	got := c.endpoints(core.Credentials{Extra: map[string]string{"kiro_auth_method": "api_key"}})
	if len(got) != len(kiroEndpoints) {
		t.Fatalf("expected %d endpoints, got %d", len(kiroEndpoints), len(got))
	}
	// API-key auth must hit the amazonaws.com surface first; kiro.dev rejects it.
	if !strings.Contains(got[0], "amazonaws.com") {
		t.Errorf("api_key should lead with amazonaws.com, got %s", got[0])
	}
}

func TestKiroEndpoints_ExternalIDPUsesRegionalAmazonSurface(t *testing.T) {
	c := NewKiro("kiro", kiroEndpoints[0])
	got := c.endpoints(core.Credentials{Extra: map[string]string{
		"kiro_auth_method": "external_idp",
		"kiro_region":      "eu-west-1",
	}})
	if !strings.Contains(got[0], "amazonaws.com") || !strings.Contains(got[0], ".eu-west-1.") {
		t.Fatalf("external_idp should lead with the regional amazon endpoint, got %v", got)
	}
}

func TestKiroEndpoints_CredentialBaseURLLeads(t *testing.T) {
	c := NewKiro("kiro", kiroEndpoints[0])
	custom := "https://relay.example.com/generateAssistantResponse"
	got := c.endpoints(core.Credentials{BaseURL: custom})
	if got[0] != custom {
		t.Fatalf("explicit base url should lead, got %s", got[0])
	}
	// The custom URL must not be duplicated among the known endpoints.
	for _, e := range got[1:] {
		if e == custom {
			t.Errorf("custom base url duplicated in fallback list")
		}
	}
}

func TestOrderAmazonFirst(t *testing.T) {
	in := []string{
		"https://runtime.us-east-1.kiro.dev/x",
		"https://codewhisperer.us-east-1.amazonaws.com/x",
		"https://q.us-east-1.amazonaws.com/x",
	}
	got := orderAmazonFirst(in)
	if !strings.Contains(got[0], "amazonaws.com") || !strings.Contains(got[1], "amazonaws.com") {
		t.Errorf("amazon hosts should come first: %v", got)
	}
	if !strings.Contains(got[2], "kiro.dev") {
		t.Errorf("non-amazon host should come last: %v", got)
	}
	// No amazon hosts: returned unchanged.
	only := []string{"https://runtime.us-east-1.kiro.dev/x"}
	if out := orderAmazonFirst(only); out[0] != only[0] {
		t.Errorf("single non-amazon host should be unchanged, got %v", out)
	}
}

func TestKiroHeaders_APIKeyMarker(t *testing.T) {
	c := NewKiro("kiro", kiroEndpoints[0])
	h := c.headers(core.Credentials{
		APIKey: "secret-key",
		Extra:  map[string]string{"kiro_auth_method": "api_key"},
	})
	if h["Authorization"] != "Bearer secret-key" {
		t.Errorf("api key should be sent as bearer, got %q", h["Authorization"])
	}
	if h["tokentype"] != "API_KEY" {
		t.Errorf("api key requests must carry tokentype=API_KEY, got %q", h["tokentype"])
	}
}

func TestKiroHeaders_OAuthHasNoTokenType(t *testing.T) {
	c := NewKiro("kiro", kiroEndpoints[0])
	h := c.headers(core.Credentials{AccessToken: "oauth-token"})
	if h["Authorization"] != "Bearer oauth-token" {
		t.Errorf("oauth should be sent as bearer, got %q", h["Authorization"])
	}
	if _, ok := h["tokentype"]; ok {
		t.Errorf("oauth requests must not carry tokentype")
	}
}

func TestKiroHeaders_ExternalIDPMarker(t *testing.T) {
	c := NewKiro("kiro", kiroEndpoints[0])
	h := c.headers(core.Credentials{
		AccessToken: "microsoft-token",
		Extra:       map[string]string{"kiro_auth_method": "external_idp"},
	})
	if h["Authorization"] != "Bearer microsoft-token" {
		t.Errorf("external identity token should be sent as bearer, got %q", h["Authorization"])
	}
	if h["TokenType"] != "EXTERNAL_IDP" {
		t.Errorf("external identity requests must carry TokenType=EXTERNAL_IDP, got %q", h["TokenType"])
	}
}

func TestKiroResolveProfileArn(t *testing.T) {
	cases := []struct {
		name  string
		creds core.Credentials
		want  string
	}{
		{
			name:  "oauth builder-id falls back to builder-id default",
			creds: core.Credentials{Extra: map[string]string{"kiro_auth_method": "builder-id"}},
			want:  kiroDefaultProfileArnBuilderID,
		},
		{
			name:  "idc without resolved arn injects no default",
			creds: core.Credentials{Extra: map[string]string{"kiro_auth_method": "idc"}},
			want:  "",
		},
		{
			name:  "imported social falls back to social default",
			creds: core.Credentials{Extra: map[string]string{"kiro_auth_method": "imported"}},
			want:  kiroDefaultProfileArnSocial,
		},
		{
			name:  "google social falls back to social default",
			creds: core.Credentials{Extra: map[string]string{"kiro_auth_method": "google"}},
			want:  kiroDefaultProfileArnSocial,
		},
		{
			name:  "no auth method falls back to builder-id default",
			creds: core.Credentials{Extra: map[string]string{}},
			want:  kiroDefaultProfileArnBuilderID,
		},
		{
			name: "resolved kiro_profile_arn wins over default",
			creds: core.Credentials{Extra: map[string]string{
				"kiro_auth_method": "builder-id",
				"kiro_profile_arn": "arn:aws:codewhisperer:us-east-1:111:profile/RESOLVED",
			}},
			want: "arn:aws:codewhisperer:us-east-1:111:profile/RESOLVED",
		},
		{
			name: "resolved profile_arn used when kiro_profile_arn empty",
			creds: core.Credentials{Extra: map[string]string{
				"kiro_auth_method": "google",
				"profile_arn":      "arn:aws:codewhisperer:us-east-1:222:profile/ALT",
			}},
			want: "arn:aws:codewhisperer:us-east-1:222:profile/ALT",
		},
		{
			name: "api_key uses only resolved arn",
			creds: core.Credentials{Extra: map[string]string{
				"kiro_auth_method": "api_key",
				"kiro_profile_arn": "arn:aws:codewhisperer:us-east-1:333:profile/KEY",
			}},
			want: "arn:aws:codewhisperer:us-east-1:333:profile/KEY",
		},
		{
			name:  "api_key without resolved arn injects no default",
			creds: core.Credentials{Extra: map[string]string{"kiro_auth_method": "api_key"}},
			want:  "",
		},
		{
			name:  "external_idp without resolved arn injects no default",
			creds: core.Credentials{Extra: map[string]string{"kiro_auth_method": "external_idp"}},
			want:  "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := kiroResolveProfileArn(tc.creds); got != tc.want {
				t.Errorf("kiroResolveProfileArn() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestKiroEndpointRetryable(t *testing.T) {

	cases := []struct {
		kind core.ErrorKind
		want bool
	}{
		{core.ErrRateLimit, false},
		{core.ErrUpstream, true},
		{core.ErrTimeout, true},
		{core.ErrAuth, false},
		{core.ErrBadRequest, false},
		{core.ErrQuotaExhausted, false},
	}
	for _, tc := range cases {
		err := &core.ProviderError{Kind: tc.kind}
		if got := kiroEndpointRetryable(err); got != tc.want {
			t.Errorf("kiroEndpointRetryable(%s) = %v, want %v", tc.kind, got, tc.want)
		}
	}
}

func TestKiroRateLimitDoesNotFailOverToAnotherHost(t *testing.T) {
	var secondCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()

	c := NewKiro("kiro", "")
	_, err := c.openStreamWithFailover(context.Background(), "model", []byte("{}"), nil,
		[]string{first.URL, second.URL})
	require.Error(t, err)
	require.Equal(t, core.ErrRateLimit, core.AsProviderError(err).Kind)
	require.Equal(t, int32(0), secondCalls.Load())
}

func TestKiroAccountSlotRejectsConcurrentRequest(t *testing.T) {
	c := NewKiro("kiro", "")
	secondConnector := NewKiro("kiro", "")
	creds := core.Credentials{AccountID: "account-1"}

	release, err := c.acquireAccountSlot(context.Background(), creds, "model")
	require.NoError(t, err)

	_, err = secondConnector.acquireAccountSlot(context.Background(), creds, "model")
	require.Error(t, err)
	pe := core.AsProviderError(err)
	require.Equal(t, core.ErrRateLimit, pe.Kind)
	require.Equal(t, 2*time.Second, pe.RetryAfter)

	release()
	releaseAgain, err := c.acquireAccountSlot(context.Background(), creds, "model")
	require.NoError(t, err)
	releaseAgain()
}

func TestKiroQuotaEndpointRetryableStopsOnRateLimit(t *testing.T) {
	require.False(t, kiroQuotaEndpointRetryable(&core.ProviderError{Kind: core.ErrRateLimit}))
	require.False(t, kiroQuotaEndpointRetryable(&core.ProviderError{Kind: core.ErrQuotaExhausted}))
	require.True(t, kiroQuotaEndpointRetryable(&core.ProviderError{Kind: core.ErrAuth}))
	require.True(t, kiroQuotaEndpointRetryable(&core.ProviderError{Kind: core.ErrUpstream}))
}

func TestKiroMetadataCachesReturnCopies(t *testing.T) {
	accountID := "cache-account"
	kiroModelCache.Delete(accountID)
	kiroQuotaCache.Delete(accountID)
	t.Cleanup(func() {
		kiroModelCache.Delete(accountID)
		kiroQuotaCache.Delete(accountID)
	})

	storeKiroModels(accountID, []ModelSpec{{ID: "model-1"}})
	models, ok := loadKiroModels(accountID)
	require.True(t, ok)
	models[0].ID = "mutated"
	modelsAgain, ok := loadKiroModels(accountID)
	require.True(t, ok)
	require.Equal(t, "model-1", modelsAgain[0].ID)

	kiroQuotaCache.Store(accountID, kiroQuotaCacheEntry{
		expiresAt: time.Now().Add(time.Minute),
		quota:     &QuotaResult{PlanName: "plan", Quotas: []QuotaEntry{{ResourceType: "credit", Limit: 100}}},
	})
	quota, ok := loadKiroQuota(accountID)
	require.True(t, ok)
	quota.Quotas[0].Limit = 0
	quotaAgain, ok := loadKiroQuota(accountID)
	require.True(t, ok)
	require.Equal(t, 100, quotaAgain.Quotas[0].Limit)
}
