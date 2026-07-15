package oauth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestNormalizeKiroExternalIDPAuth(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"preferred_username": "user@example.com",
		"exp":                4_102_444_800,
	})
	jwt := "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
	raw, _ := json.Marshal(map[string]any{"json": `{
		"auth_method":"external_idp",
		"access_token":"` + jwt + `",
		"refresh_token":"refresh-token",
		"client_id":"client-id",
		"token_endpoint":"https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
		"profile_arn":"arn:aws:codewhisperer:us-east-1:123:profile/ABC",
		"region":"us-east-1",
		"scopes":["offline_access","api://client/conversations"]
	}`})

	tokens, err := NormalizeKiroExternalIDPAuth(raw)
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != jwt || tokens.RefreshToken != "refresh-token" {
		t.Fatalf("tokens were not normalized: %+v", tokens)
	}
	if tokens.Email != "user@example.com" {
		t.Fatalf("email = %q", tokens.Email)
	}
	if tokens.Extra["kiro_auth_method"] != "external_idp" || tokens.Extra["kiro_scope"] != "offline_access api://client/conversations" {
		t.Fatalf("metadata was not normalized: %+v", tokens.Extra)
	}
}

func TestNormalizeKiroExternalIDPAuthRejectsEndpoint(t *testing.T) {
	raw := []byte(`{"auth_method":"external_idp","access_token":"a","refresh_token":"r","client_id":"c","token_endpoint":"https://example.com/token","profile_arn":"arn","scopes":"offline_access"}`)
	if _, err := NormalizeKiroExternalIDPAuth(raw); err == nil || !strings.Contains(err.Error(), "Microsoft") {
		t.Fatalf("expected Microsoft endpoint error, got %v", err)
	}
}

func TestKiroExternalIDPRefresh(t *testing.T) {
	original := httpClient
	t.Cleanup(func() { httpClient = original })
	httpClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host != "login.microsoftonline.com" {
			t.Fatalf("unexpected endpoint: %s", req.URL)
		}
		if req.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("unexpected content type: %s", req.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(req.Body)
		values := string(body)
		for _, expected := range []string{"grant_type=refresh_token", "client_id=client-id", "refresh_token=old-token", "scope=offline_access"} {
			if !strings.Contains(values, expected) {
				t.Fatalf("refresh body %q missing %q", values, expected)
			}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"access_token":"new-token","refresh_token":"new-refresh","expires_in":3600}`)),
		}, nil
	})}

	tokens, err := KiroExternalIDPRefresh(context.Background(), "old-token", map[string]string{
		"kiro_client_id":      "client-id",
		"kiro_token_endpoint": "https://login.microsoftonline.com/tenant/oauth2/v2.0/token",
		"kiro_scope":          "offline_access",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tokens.AccessToken != "new-token" || tokens.RefreshToken != "new-refresh" || tokens.ExpiresIn != 3600 {
		t.Fatalf("unexpected refreshed tokens: %+v", tokens)
	}
}
