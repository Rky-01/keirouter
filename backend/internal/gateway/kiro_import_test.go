package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mydisha/keirouter/backend/internal/store"
)

func TestKiroImportCLIProxyPersistsExternalIDPCredential(t *testing.T) {
	s, db := newBulkTestServer(t)
	authJSON := `{"auth_method":"external_idp","access_token":"access-token","refresh_token":"refresh-token","client_id":"client-id","token_endpoint":"https://login.microsoftonline.com/tenant/oauth2/v2.0/token","profile_arn":"arn:aws:codewhisperer:us-east-1:123:profile/ABC","region":"us-east-1","scopes":["openid","offline_access"]}`
	body, err := json.Marshal(map[string]string{"json": authJSON})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/kiro/import-cli-proxy", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	s.kiroImportCLIProxy(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	accounts, err := db.Accounts().ListByProvider(context.Background(), adminTenant, "kiro")
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, store.AuthOAuth, accounts[0].AuthKind)
	require.NotNil(t, accounts[0].TokenExpiresAt)

	creds, err := s.vault.Open(accounts[0])
	require.NoError(t, err)
	require.Equal(t, "access-token", creds.AccessToken)
	refreshToken, err := s.vault.OpenRefreshToken(accounts[0])
	require.NoError(t, err)
	require.Equal(t, "refresh-token", refreshToken)
	require.Equal(t, "external_idp", creds.Extra["kiro_auth_method"])
	require.Equal(t, "client-id", creds.Extra["kiro_client_id"])
	require.Equal(t, "openid offline_access", creds.Extra["kiro_scope"])
}
