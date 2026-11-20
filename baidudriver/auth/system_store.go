package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	keyring "github.com/zalando/go-keyring"
)

const (
	systemCredentialVersion = 1
	maxSystemCredentialSize = 2048
)

type storedCredential struct {
	Version    int        `json:"version"`
	Credential Credential `json:"credential"`
}

type keyringBackend interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type operatingSystemKeyring struct{}

func (operatingSystemKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (operatingSystemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (operatingSystemKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

// SystemStore stores credentials in macOS Keychain, Windows Credential
// Manager, or the Linux Secret Service through the current desktop session.
type SystemStore struct {
	service string
	backend keyringBackend
}

// NewSystemStore creates an operating system credential store. A missing or
// unavailable platform service is reported by Load or Save; the SDK never
// silently falls back to a plaintext file.
func NewSystemStore(service string) (*SystemStore, error) {
	service = strings.TrimSpace(service)
	if service == "" || len(service) > 256 || strings.ContainsAny(service, "\r\n") {
		return nil, errors.New("baidupan auth: keyring service must be 1-256 characters without line breaks")
	}
	return &SystemStore{service: service, backend: operatingSystemKeyring{}}, nil
}

func (s *SystemStore) Load(ctx context.Context, key string) (*Credential, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	payload, err := s.backend.Get(s.service, key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrCredentialNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("baidupan auth: load credential from system store: %w", err)
	}
	if len(payload) > maxSystemCredentialSize {
		return nil, errors.New("baidupan auth: system store credential exceeds the 2048-byte limit")
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	var stored storedCredential
	if err := json.Unmarshal([]byte(payload), &stored); err != nil {
		return nil, errors.New("baidupan auth: system store contains an invalid credential")
	}
	if stored.Version != systemCredentialVersion {
		return nil, errors.New("baidupan auth: system store credential has an unsupported version")
	}
	if strings.TrimSpace(stored.Credential.AccessToken) == "" {
		return nil, errors.New("baidupan auth: system store credential has no access token")
	}
	return &stored.Credential, nil
}

func (s *SystemStore) Save(ctx context.Context, key string, credential *Credential) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(key) == "" || credential == nil || strings.TrimSpace(credential.AccessToken) == "" {
		return errors.New("baidupan auth: store key and access token are required")
	}
	payload, err := json.Marshal(storedCredential{Version: systemCredentialVersion, Credential: *credential})
	if err != nil {
		return errors.New("baidupan auth: encode credential for system store")
	}
	if len(payload) > maxSystemCredentialSize {
		return errors.New("baidupan auth: credential exceeds the 2048-byte system store limit")
	}
	if err := s.backend.Set(s.service, key, string(payload)); err != nil {
		return fmt.Errorf("baidupan auth: save credential to system store: %w", err)
	}
	return contextError(ctx)
}

func (s *SystemStore) Delete(ctx context.Context, key string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if err := s.backend.Delete(s.service, key); errors.Is(err, keyring.ErrNotFound) {
		return ErrCredentialNotFound
	} else if err != nil {
		return fmt.Errorf("baidupan auth: delete credential from system store: %w", err)
	}
	return contextError(ctx)
}
