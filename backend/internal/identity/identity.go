// Package identity issues and verifies KeiRouter API keys.
//
// Keys are never stored in plaintext: creation returns the plaintext once for
// the user to copy, while the store keeps only an argon2id verifier plus a
// fast SHA-256 lookup index. Authentication looks up the candidate row by the
// lookup index, then confirms with a constant-time argon2 comparison.
package identity

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/mydisha/keirouter/backend/internal/crypto"
	"github.com/mydisha/keirouter/backend/internal/store"
)

// ErrUnauthorized is returned when a presented key is invalid or disabled.
var ErrUnauthorized = errors.New("identity: unauthorized")

// Service manages API key lifecycle and authentication.
type Service struct {
	keys *store.APIKeyRepo
}

// New builds an identity Service.
func New(keys *store.APIKeyRepo) *Service {
	return &Service{keys: keys}
}

// Issued is the result of creating a key. Plaintext is shown exactly once.
type Issued struct {
	Record    store.APIKey
	Plaintext string
}

// Create mints a new API key for a tenant/project, persisting only its hashes.
func (s *Service) Create(ctx context.Context, tenantID, projectID, name string) (Issued, error) {
	gen, err := crypto.GenerateAPIKey()
	if err != nil {
		return Issued{}, err
	}
	rec := store.APIKey{
		ID:         uuid.NewString(),
		TenantID:   tenantID,
		ProjectID:  projectID,
		Name:       name,
		KeyHash:    gen.Hash,
		LookupHash: gen.Lookup,
		Display:    gen.Display,
		CreatedAt:  time.Now(),
	}
	if err := s.keys.Create(ctx, rec); err != nil {
		return Issued{}, err
	}
	return Issued{Record: rec, Plaintext: gen.Plaintext}, nil
}

// Authenticate verifies a presented plaintext key and returns its record. On
// success it best-effort updates the key's last-used timestamp.
func (s *Service) Authenticate(ctx context.Context, plaintext string) (store.APIKey, error) {
	if plaintext == "" {
		return store.APIKey{}, ErrUnauthorized
	}
	lookup := crypto.LookupHash(plaintext)

	rec, err := s.keys.FindByLookup(ctx, lookup)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return store.APIKey{}, ErrUnauthorized
		}
		return store.APIKey{}, err
	}
	if rec.Disabled {
		return store.APIKey{}, ErrUnauthorized
	}

	ok, err := crypto.VerifyAPIKey(plaintext, rec.KeyHash)
	if err != nil || !ok {
		return store.APIKey{}, ErrUnauthorized
	}

	// Best-effort last-used update; failure here must not fail auth.
	_ = s.keys.TouchLastUsed(ctx, rec.ID, time.Now())
	return rec, nil
}

// List returns the keys for a tenant (without secrets).
func (s *Service) List(ctx context.Context, tenantID string) ([]store.APIKey, error) {
	return s.keys.List(ctx, tenantID)
}

// SetDisabled toggles a key's disabled state.
func (s *Service) SetDisabled(ctx context.Context, id string, disabled bool) error {
	return s.keys.SetDisabled(ctx, id, disabled)
}

// Delete removes a key.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.keys.Delete(ctx, id)
}