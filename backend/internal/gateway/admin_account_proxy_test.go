package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/mydisha/keirouter/backend/internal/store"
	"github.com/mydisha/keirouter/backend/internal/vault"
)

func TestAdminUpdateAccountProxyPool(t *testing.T) {
	s, db := newBulkTestServer(t)
	s.pools = db.ProxyPools()
	now := time.Now()
	pool := store.ProxyPool{
		ID: uuid.NewString(), Name: "relay", Type: "cloudflare",
		ProxyURL: "https://relay.example.com", IsActive: true,
		TestStatus: "unknown", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, s.pools.Create(context.Background(), pool))
	acc := store.Account{
		ID: uuid.NewString(), TenantID: adminTenant, Provider: "openai",
		Label: "account", AuthKind: store.AuthAPIKey, Priority: 100,
		CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, s.vault.Seal(&acc, vault.NewSecret{APIKey: "secret", Metadata: map[string]string{}}))
	require.NoError(t, s.accounts.Create(context.Background(), acc))

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", acc.ID)
	req := httptest.NewRequest(http.MethodPatch, "/accounts/"+acc.ID, strings.NewReader(`{"proxy_pool_id":"`+pool.ID+`"}`))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	s.adminUpdateAccount(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	updated, err := s.accounts.Get(context.Background(), acc.ID)
	require.NoError(t, err)
	require.Equal(t, pool.ID, updated.ProxyPoolID)
}
