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
	"net/http"
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