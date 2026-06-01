// Package connectors implements provider drivers: the components that render a
// canonical request to a provider's wire format, perform the HTTP call, and
// parse the response (unary or streaming) back into canonical chunks.
//
// Connectors are thin and stateless. They delegate format translation to the
// transform package and focus on transport: URL construction, auth headers,
// streaming, and mapping HTTP/transport failures to structured ProviderErrors
// that drive the dispatcher's fallback decisions.
package connectors

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// sharedClient is reused across connectors; the transport pools connections.
var sharedClient = &http.Client{
	Timeout: 0, // per-request deadlines come from context
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// doJSON performs a JSON POST and returns the response body, mapping transport
// and HTTP errors to structured ProviderErrors.
func doJSON(ctx context.Context, provider, model, url string, body []byte, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, transportError(ctx, provider, model, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: provider, Model: model, Message: "read body: " + err.Error(), Cause: err}
	}

	if resp.StatusCode >= 400 {
		return nil, httpStatusError(provider, model, resp, respBody)
	}
	return respBody, nil
}

// doJSONMethod performs a JSON request with an explicit method (GET/POST) and
// returns the response body. A nil body sends no payload (for GET).
func doJSONMethod(ctx context.Context, method, provider, model, url string, body []byte, headers map[string]string) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, transportError(ctx, provider, model, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: provider, Model: model, Message: "read body: " + err.Error(), Cause: err}
	}
	if resp.StatusCode >= 400 {
		return nil, httpStatusError(provider, model, resp, respBody)
	}
	return respBody, nil
}

// doFormPOST performs an application/x-www-form-urlencoded POST and returns the
// response body, mapping transport and HTTP errors to ProviderErrors. Used for
// OAuth token endpoints (refresh, JWT-bearer assertion exchange).
func doFormPOST(ctx context.Context, provider, model, endpoint string, form url.Values, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, transportError(ctx, provider, model, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: provider, Model: model, Message: "read body: " + err.Error(), Cause: err}
	}
	if resp.StatusCode >= 400 {
		return nil, httpStatusError(provider, model, resp, respBody)
	}
	return respBody, nil
}

// rawResponse carries non-JSON response bytes plus the upstream content type,
// used by binary endpoints like text-to-speech.
type rawResponse struct {
	Body        []byte
	ContentType string
}

// doRaw performs a JSON POST but returns the raw response bytes and content
// type instead of parsing JSON. Used for endpoints that return binary audio.
func doRaw(ctx context.Context, provider, model, url string, body []byte, headers map[string]string) (*rawResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, transportError(ctx, provider, model, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: provider, Model: model, Message: "read body: " + err.Error(), Cause: err}
	}
	if resp.StatusCode >= 400 {
		return nil, httpStatusError(provider, model, resp, respBody)
	}
	return &rawResponse{Body: respBody, ContentType: resp.Header.Get("Content-Type")}, nil
}

// multipartField is one non-file form field in a multipart upload.
type multipartField struct{ Name, Value string }

// doMultipart performs a multipart/form-data POST with a single file part plus
// extra text fields, returning the JSON response body. Used by speech-to-text.
func doMultipart(ctx context.Context, provider, model, url, fileField, filename string, fileData []byte, fields []multipartField, headers map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	fw, err := mw.CreateFormFile(fileField, filename)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	if _, err := fw.Write(fileData); err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	for _, f := range fields {
		if f.Value == "" {
			continue
		}
		if err := mw.WriteField(f.Name, f.Value); err != nil {
			return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
		}
	}
	if err := mw.Close(); err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, transportError(ctx, provider, model, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrUpstream, Provider: provider, Model: model, Message: "read body: " + err.Error(), Cause: err}
	}
	if resp.StatusCode >= 400 {
		return nil, httpStatusError(provider, model, resp, respBody)
	}
	return respBody, nil
}

// openStream performs a streaming POST and returns the response for the caller
// to read SSE lines from. The caller must close resp.Body.
func openStream(ctx context.Context, provider, model, url string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, &core.ProviderError{Kind: core.ErrInternal, Provider: provider, Model: model, Message: err.Error(), Cause: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, transportError(ctx, provider, model, err)
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, httpStatusError(provider, model, resp, errBody)
	}
	return resp, nil
}

// streamParser is the subset of a codec the SSE pump needs: turning one
// upstream SSE data payload into canonical chunks.
type streamParser interface {
	ParseStreamLine(line []byte, model string) ([]core.StreamChunk, error)
}

// scanOpenAISSE consumes an OpenAI-style SSE response, parsing each "data:"
// payload through the given codec and emitting canonical chunks on the returned
// channel. It owns resp.Body and closes it when done. Shared by the
// OpenAI-compatible subscription connectors (Qwen, iFlow, ...) to avoid
// duplicating the streaming goroutine.
func scanOpenAISSE(ctx context.Context, provider, model string, resp *http.Response, codec streamParser) <-chan core.StreamChunk {
	out := make(chan core.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		scanner := sseScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			payload, ok := parseSSEData(scanner.Text())
			if !ok {
				continue
			}
			chunks, perr := codec.ParseStreamLine([]byte(payload), model)
			if perr != nil {
				continue
			}
			for _, ch := range chunks {
				select {
				case out <- ch:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			out <- core.StreamChunk{
				Type: core.ChunkError,
				Err:  &core.ProviderError{Kind: core.ErrTimeout, Provider: provider, Model: model, Message: err.Error(), Cause: err},
			}
		}
	}()
	return out
}

// sseScanner returns a bufio.Scanner configured for SSE: it reads one logical
// line at a time with a generous buffer for large data payloads.
func sseScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	return sc
}

// parseSSEData extracts the payload from an SSE "data:" line, or returns ("",
// false) for non-data lines (comments, event:, blank).
func parseSSEData(line string) (string, bool) {
	line = strings.TrimRight(line, "\r")
	if !strings.HasPrefix(line, "data:") {
		return "", false
	}
	return strings.TrimSpace(strings.TrimPrefix(line, "data:")), true
}

// transportError classifies a transport-level failure (DNS, connection, ctx).
func transportError(ctx context.Context, provider, model string, err error) error {
	kind := core.ErrUpstream
	if ctx.Err() == context.DeadlineExceeded {
		kind = core.ErrTimeout
	}
	return &core.ProviderError{Kind: kind, Provider: provider, Model: model, Message: err.Error(), Cause: err}
}

// httpStatusError maps an HTTP error status to a structured ProviderError.
func httpStatusError(provider, model string, resp *http.Response, body []byte) error {
	kind := core.ErrUpstream
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		kind = core.ErrRateLimit
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		kind = core.ErrAuth
	case resp.StatusCode == http.StatusPaymentRequired:
		kind = core.ErrQuotaExhausted
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		kind = core.ErrBadRequest
	}

	pe := &core.ProviderError{
		Kind:       kind,
		Provider:   provider,
		Model:      model,
		StatusCode: resp.StatusCode,
		Message:    truncateError(body),
	}
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			pe.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	return pe
}

func truncateError(body []byte) string {
	const max = 512
	s := strings.TrimSpace(string(body))
	if len(s) > max {
		return s[:max] + "…"
	}
	if s == "" {
		return "upstream returned an error with empty body"
	}
	return s
}

// bearer builds an Authorization: Bearer header value.
func bearer(token string) string { return "Bearer " + token }

// mergeHeaders combines connector defaults with credential-supplied headers.
func mergeHeaders(base map[string]string, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// joinURL concatenates a base URL and path, collapsing duplicate slashes.
func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/%s", base, path)
}