package connectors

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mydisha/keirouter/backend/internal/core"
)

// CloudflareModelSource implements LiveModelSource for Cloudflare Workers AI.
// Cloudflare's model listing endpoint returns a non-standard envelope
// ({"success":true,"result":[...]}) unlike the OpenAI {"data":[...]} shape,
// so it needs a dedicated parser.
type CloudflareModelSource struct {
	defaultBase string
}

// ListModels fetches available models from the Cloudflare Workers AI API.
// The endpoint is GET /accounts/{accountId}/ai/v1/models, authenticated with
// the bearer token. The response uses Cloudflare's envelope format.
func (s *CloudflareModelSource) ListModels(ctx context.Context, creds core.Credentials) ([]ModelSpec, error) {
	base := s.defaultBase
	if creds.BaseURL != "" {
		base = creds.BaseURL
	}
	// Resolve {accountId} placeholder from creds.Extra.
	for key, val := range creds.Extra {
		base = strings.ReplaceAll(base, "{"+key+"}", val)
	}
	// If the base URL still contains an unresolved placeholder, we can't
	// discover models — the account ID is required.
	if strings.Contains(base, "{") {
		return nil, fmt.Errorf("cloudflare: account ID not available for model discovery")
	}

	// Cloudflare uses POST /models/search, not GET /models.
	url := joinURL(base, "models/search")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, err
	}
	switch {
	case creds.AccessToken != "":
		req.Header.Set("Authorization", bearer(creds.AccessToken))
	case creds.APIKey != "":
		req.Header.Set("Authorization", bearer(creds.APIKey))
	}
	req.Header.Set("Accept", "application/json")

	resp, err := sharedClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("GET /models returned %d: %s", resp.StatusCode, truncateError(body))
	}

	// Cloudflare returns: {"success":true,"result":[{"id":"...","name":"..."},...]}
	// Some endpoints may also return the OpenAI shape {"data":[...]}, so we
	// parse both formats for resilience.
	var raw struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
		Result []struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"result"`
		// OpenAI-compatible fallback
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode cloudflare /models response: %w", err)
	}

	// If Cloudflare reported an error, surface it.
	if !raw.Success && len(raw.Errors) > 0 {
		return nil, fmt.Errorf("cloudflare: %s", raw.Errors[0].Message)
	}

	out := make([]ModelSpec, 0, len(raw.Result)+len(raw.Data))

	// Parse Cloudflare-format result array.
	for _, entry := range raw.Result {
		if entry.ID == "" {
			continue
		}
		name := entry.Name
		if name == "" {
			name = entry.ID
		}
		kind := core.ServiceLLM
		// Detect image models by their path prefix.
		if strings.HasPrefix(entry.ID, "@cf/black-forest-labs/") ||
			strings.HasPrefix(entry.ID, "@cf/stabilityai/") {
			kind = core.ServiceImage
		}
		out = append(out, ModelSpec{ID: entry.ID, Name: name, Kind: kind})
	}

	// Parse OpenAI-format data array (fallback).
	for _, entry := range raw.Data {
		if entry.ID == "" {
			continue
		}
		out = append(out, ModelSpec{ID: entry.ID, Name: entry.ID, Kind: core.ServiceLLM})
	}

	return out, nil
}