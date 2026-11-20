package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

// ErrCredentialNotFound indicates that no credential exists for a store key.
var ErrCredentialNotFound = errors.New("baidupan auth: credential not found")

// Credential contains the complete token response stored on the local
// device. Callers should treat every string field as secret and avoid logging
// the value.
type Credential struct {
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token,omitempty"`
	ExpiresIn       int64     `json:"expires_in,omitempty"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	Scope           string    `json:"scope,omitempty"`
	RequestedScope  string    `json:"requested_scope,omitempty"`
	AuthorizedPaths []string  `json:"authorized_paths,omitempty"`
	// RedirectURI is the value echoed by the token endpoint. It may include the
	// one-time state parameter and must not be logged.
	RedirectURI   string                     `json:"redirect_uri,omitempty"`
	TokenType     string                     `json:"token_type,omitempty"`
	SessionKey    string                     `json:"session_key,omitempty"`
	SessionSecret string                     `json:"session_secret,omitempty"`
	RequestID     string                     `json:"request_id,omitempty"`
	ServerTime    int64                      `json:"server_time,omitempty"`
	CreatedAt     time.Time                  `json:"created_at"`
	Extra         map[string]json.RawMessage `json:"extra,omitempty"`
}

// CredentialStore persists OAuth credentials. Implementations must replace a
// credential atomically for a given key.
type CredentialStore interface {
	Load(ctx context.Context, key string) (*Credential, error)
	Save(ctx context.Context, key string, credential *Credential) error
	Delete(ctx context.Context, key string) error
}

// MemoryStore is an explicit non-persistent store for tests and short-lived
// processes. It is safe for concurrent use.
type MemoryStore struct {
	mu          sync.RWMutex
	credentials map[string]*Credential
}

// NewMemoryStore creates an empty in-memory credential store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{credentials: make(map[string]*Credential)}
}

func (s *MemoryStore) Load(ctx context.Context, key string) (*Credential, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	credential, ok := s.credentials[key]
	s.mu.RUnlock()
	if !ok {
		return nil, ErrCredentialNotFound
	}
	return cloneCredential(credential), nil
}

func (s *MemoryStore) Save(ctx context.Context, key string, credential *Credential) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" || credential == nil || strings.TrimSpace(credential.AccessToken) == "" {
		return errors.New("baidupan auth: store key and access token are required")
	}
	s.mu.Lock()
	if s.credentials == nil {
		s.credentials = make(map[string]*Credential)
	}
	s.credentials[key] = cloneCredential(credential)
	s.mu.Unlock()
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.credentials[key]; !ok {
		return ErrCredentialNotFound
	}
	delete(s.credentials, key)
	return nil
}

func credentialStoreKey(appKey, redirectURI, scope string) string {
	sum := sha256.Sum256([]byte(appKey + "\x00" + redirectURI + "\x00" + scope + "\x00directory"))
	return "directory-oauth-v1-" + hex.EncodeToString(sum[:])
}

func cloneCredential(credential *Credential) *Credential {
	if credential == nil {
		return nil
	}
	copy := *credential
	copy.AuthorizedPaths = append([]string(nil), credential.AuthorizedPaths...)
	if credential.Extra != nil {
		copy.Extra = make(map[string]json.RawMessage, len(credential.Extra))
		for key, value := range credential.Extra {
			copy.Extra[key] = append(json.RawMessage(nil), value...)
		}
	}
	return &copy
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("baidupan auth: context must not be nil")
	}
	return ctx.Err()
}
