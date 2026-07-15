package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestForeignKiroAPIKeyUsesExportedAccessToken(t *testing.T) {
	s, db := newBulkTestServer(t)
	payload := []byte(`[{"id":"source-1","provider":"kiro","authType":"api_key","name":"Kiro key","accessToken":"headless-token","providerSpecificData":{"authMethod":"api_key","profileArn":"arn:aws:codewhisperer:us-east-1:123:profile/ABC","region":"us-east-1"}}]`)
	doc := map[string]json.RawMessage{"providerConnections": payload}
	result := &foreignImportResult{}
	s.importN9routerConnections(context.Background(), doc, result, nil)
	require.Equal(t, 1, result.Accounts, result.Errors)

	accounts, err := db.Accounts().ListByProvider(context.Background(), adminTenant, "kiro")
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	creds, err := s.vault.Open(accounts[0])
	require.NoError(t, err)
	require.Equal(t, "headless-token", creds.APIKey)
	require.Equal(t, "api_key", creds.Extra["kiro_auth_method"])
	require.Equal(t, "arn:aws:codewhisperer:us-east-1:123:profile/ABC", creds.Extra["kiro_profile_arn"])
}

func TestForeignKiroExternalIDPPreservesBearerAndRefreshConfig(t *testing.T) {
	s, db := newBulkTestServer(t)
	payload := []byte(`[{"id":"source-2","provider":"kiro","authType":"oauth","email":"person@example.com","accessToken":"bearer-token","refreshToken":"refresh-token","providerSpecificData":{"authMethod":"external_idp","profileArn":"arn:aws:codewhisperer:eu-west-1:123:profile/XYZ","region":"eu-west-1","clientId":"client-id","tokenEndpoint":"https://login.microsoftonline.com/tenant/oauth2/v2.0/token","scope":"openid offline_access"}}]`)
	doc := map[string]json.RawMessage{"providerConnections": payload}
	result := &foreignImportResult{}
	s.importN9routerConnections(context.Background(), doc, result, nil)
	require.Equal(t, 1, result.Accounts, result.Errors)

	accounts, err := db.Accounts().ListByProvider(context.Background(), adminTenant, "kiro")
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	creds, err := s.vault.Open(accounts[0])
	require.NoError(t, err)
	require.Equal(t, "bearer-token", creds.AccessToken)
	require.Equal(t, "external_idp", creds.Extra["kiro_auth_method"])
	require.Equal(t, "client-id", creds.Extra["kiro_client_id"])
	require.Equal(t, "eu-west-1", creds.Extra["kiro_region"])

	refreshToken, err := s.vault.OpenRefreshToken(accounts[0])
	require.NoError(t, err)
	require.Equal(t, "refresh-token", refreshToken)
}
