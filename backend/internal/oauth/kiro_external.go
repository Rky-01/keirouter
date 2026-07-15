package oauth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const kiroExternalDefaultExpiry = 3600

var microsoftTokenEndpointHosts = map[string]bool{
	"login.microsoftonline.com": true,
	"login.microsoft.com":       true,
	"login.windows.net":         true,
}

func NormalizeKiroExternalIDPAuth(raw []byte) (*Tokens, error) {
	var input any
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil, fmt.Errorf("kiro: CLI proxy auth JSON is invalid")
	}
	for {
		switch value := input.(type) {
		case string:
			if err := json.Unmarshal([]byte(value), &input); err != nil {
				return nil, fmt.Errorf("kiro: CLI proxy auth JSON is invalid")
			}
			continue
		case map[string]any:
			for _, key := range []string{"cliProxyAuth", "auth", "json"} {
				if nested, ok := value[key]; ok {
					input = nested
					goto unwrap
				}
			}
			return normalizeKiroExternalMap(value)
		default:
			return nil, fmt.Errorf("kiro: CLI proxy auth JSON is required")
		}
	unwrap:
	}
}

func normalizeKiroExternalMap(input map[string]any) (*Tokens, error) {
	authMethod := firstString(input, "auth_method", "authMethod")
	if authMethod != "" && authMethod != "external_idp" {
		return nil, fmt.Errorf("kiro: only external_idp auth is supported by this importer")
	}
	accessToken := firstString(input, "access_token", "accessToken")
	refreshToken := firstString(input, "refresh_token", "refreshToken")
	clientID := firstString(input, "client_id", "clientId")
	tokenEndpoint, err := validateMicrosoftTokenEndpoint(firstString(input, "token_endpoint", "tokenEndpoint"))
	if err != nil {
		return nil, err
	}
	profileARN := firstString(input, "profile_arn", "profileArn")
	region := firstString(input, "region")
	if region == "" {
		region = kiroDefaultRegion
	}
	scope := normalizeKiroScope(firstValue(input, "scopes", "scope"))

	for field, value := range map[string]string{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"client_id":     clientID,
		"scopes":        scope,
		"profile_arn":   profileARN,
	} {
		if value == "" {
			return nil, fmt.Errorf("kiro: %s is required", field)
		}
	}

	expiresIn := externalExpiresIn(input, accessToken)
	email := firstString(input, "email")
	if email == "" {
		email = externalJWTIdentity(accessToken)
	}
	return &Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		Email:        email,
		Extra: map[string]string{
			"kiro_auth_method":    "external_idp",
			"kiro_profile_arn":    profileARN,
			"kiro_region":         region,
			"kiro_client_id":      clientID,
			"kiro_token_endpoint": tokenEndpoint,
			"kiro_scope":          scope,
		},
	}, nil
}

func KiroExternalIDPRefresh(ctx context.Context, refreshToken string, metadata map[string]string) (*Tokens, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, fmt.Errorf("kiro: no refresh token")
	}
	clientID := strings.TrimSpace(metadata["kiro_client_id"])
	if clientID == "" {
		return nil, fmt.Errorf("kiro: client id is required for external_idp refresh")
	}
	tokenEndpoint, err := validateMicrosoftTokenEndpoint(metadata["kiro_token_endpoint"])
	if err != nil {
		return nil, err
	}
	scope := normalizeKiroScope(metadata["kiro_scope"])
	if scope == "" {
		return nil, fmt.Errorf("kiro: scope is required for external_idp refresh")
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {refreshToken},
		"scope":         {scope},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("kiro: build external_idp refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kiro: external_idp refresh request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("kiro: read external_idp refresh response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("kiro: external_idp refresh failed (%d): %s", resp.StatusCode, truncate(raw, 300))
	}
	var parsed struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("kiro: parse external_idp refresh response: %w", err)
	}
	if strings.TrimSpace(parsed.AccessToken) == "" {
		return nil, fmt.Errorf("kiro: external_idp refresh response missing access token")
	}
	if parsed.RefreshToken == "" {
		parsed.RefreshToken = refreshToken
	}
	if parsed.ExpiresIn <= 0 {
		parsed.ExpiresIn = kiroExternalDefaultExpiry
	}
	return &Tokens{AccessToken: parsed.AccessToken, RefreshToken: parsed.RefreshToken, ExpiresIn: parsed.ExpiresIn}, nil
}

func validateMicrosoftTokenEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("kiro: token_endpoint is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("kiro: token_endpoint must be a valid HTTPS URL")
	}
	if parsed.User != nil || !microsoftTokenEndpointHosts[strings.ToLower(parsed.Hostname())] {
		return "", fmt.Errorf("kiro: token_endpoint must be a Microsoft login endpoint")
	}
	return parsed.String(), nil
}

func normalizeKiroScope(value any) string {
	switch scope := value.(type) {
	case string:
		return strings.Join(strings.Fields(scope), " ")
	case []any:
		parts := make([]string, 0, len(scope))
		for _, entry := range scope {
			if item, ok := entry.(string); ok && strings.TrimSpace(item) != "" {
				parts = append(parts, strings.TrimSpace(item))
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func externalExpiresIn(input map[string]any, accessToken string) int {
	now := time.Now()
	if value := firstValue(input, "expired", "expires_at", "expiresAt"); value != nil {
		var expiry time.Time
		switch typed := value.(type) {
		case string:
			expiry, _ = time.Parse(time.RFC3339, typed)
			if expiry.IsZero() {
				if n, err := strconv.ParseInt(typed, 10, 64); err == nil {
					expiry = unixExpiry(n)
				}
			}
		case float64:
			expiry = unixExpiry(int64(typed))
		}
		if !expiry.IsZero() {
			return max(1, int(time.Until(expiry).Seconds()))
		}
	}
	if value := firstValue(input, "expires_in", "expiresIn"); value != nil {
		switch typed := value.(type) {
		case float64:
			if typed > 0 {
				return int(typed)
			}
		case string:
			if n, err := strconv.Atoi(typed); err == nil && n > 0 {
				return n
			}
		}
	}
	if payload := decodeExternalJWT(accessToken); payload != nil {
		if exp, ok := payload["exp"].(float64); ok {
			return max(1, int(time.Unix(int64(exp), 0).Sub(now).Seconds()))
		}
	}
	return kiroExternalDefaultExpiry
}

func unixExpiry(value int64) time.Time {
	if value > 1_000_000_000_000 {
		return time.UnixMilli(value)
	}
	return time.Unix(value, 0)
}

func externalJWTIdentity(token string) string {
	payload := decodeExternalJWT(token)
	for _, key := range []string{"email", "preferred_username", "upn", "sub"} {
		if value, ok := payload[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func decodeExternalJWT(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var payload map[string]any
	if json.NewDecoder(bytes.NewReader(raw)).Decode(&payload) != nil {
		return nil
	}
	return payload
}

func firstString(input map[string]any, keys ...string) string {
	value := firstValue(input, keys...)
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func firstValue(input map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := input[key]; ok && value != nil {
			return value
		}
	}
	return nil
}
