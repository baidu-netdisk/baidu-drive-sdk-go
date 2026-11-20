package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	keyring "github.com/zalando/go-keyring"
)

type fakeKeyringBackend struct {
	get    func(service, user string) (string, error)
	set    func(service, user, password string) error
	delete func(service, user string) error
}

func (f *fakeKeyringBackend) Get(service, user string) (string, error) {
	if f.get == nil {
		return "", keyring.ErrNotFound
	}
	return f.get(service, user)
}

func (f *fakeKeyringBackend) Set(service, user, password string) error {
	if f.set == nil {
		return nil
	}
	return f.set(service, user, password)
}

func (f *fakeKeyringBackend) Delete(service, user string) error {
	if f.delete == nil {
		return keyring.ErrNotFound
	}
	return f.delete(service, user)
}

func TestNewSystemStoreValidation(t *testing.T) {
	t.Parallel()

	for _, service := range []string{"", "  ", "line\nbreak", strings.Repeat("s", 257)} {
		if _, err := NewSystemStore(service); err == nil {
			t.Errorf("NewSystemStore(%q) unexpectedly succeeded", service)
		}
	}
	store, err := NewSystemStore("  baidu-test  ")
	if err != nil {
		t.Fatalf("NewSystemStore() error = %v", err)
	}
	if store.service != "baidu-test" {
		t.Errorf("service = %q", store.service)
	}
	if _, ok := store.backend.(operatingSystemKeyring); !ok {
		t.Errorf("backend type = %T", store.backend)
	}
}

func TestSystemStoreBackendRoundTrip(t *testing.T) {
	t.Parallel()

	var payload string
	var deleted bool
	backend := &fakeKeyringBackend{
		set: func(service, user, password string) error {
			if service != "baidu-test" || user != "account-a" {
				t.Errorf("Set(%q, %q, ...)", service, user)
			}
			payload = password
			return nil
		},
		get: func(service, user string) (string, error) {
			if service != "baidu-test" || user != "account-a" {
				t.Errorf("Get(%q, %q)", service, user)
			}
			return payload, nil
		},
		delete: func(service, user string) error {
			if service != "baidu-test" || user != "account-a" {
				t.Errorf("Delete(%q, %q)", service, user)
			}
			deleted = true
			return nil
		},
	}
	store := &SystemStore{service: "baidu-test", backend: backend}
	want := &Credential{
		AccessToken:     "access-token",
		RefreshToken:    "refresh-token",
		ExpiresIn:       3600,
		ExpiresAt:       time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Scope:           "nd_open_read,nd_open_write",
		RequestedScope:  defaultScope,
		AuthorizedPaths: []string{"/apps/MyApp/images", "/apps/MyApp/docs"},
		RedirectURI:     "http://127.0.0.1:8787/oauth/callback?state=stored-state",
		CreatedAt:       time.Date(2026, 8, 31, 23, 0, 0, 0, time.UTC),
		Extra:           map[string]json.RawMessage{"custom": json.RawMessage(`true`)},
	}
	if err := store.Save(context.Background(), "account-a", want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if strings.Contains(payload, "client_secret") {
		t.Fatalf("stored payload unexpectedly contains client_secret: %s", payload)
	}
	var envelope storedCredential
	if err := json.Unmarshal([]byte(payload), &envelope); err != nil {
		t.Fatalf("stored payload is invalid JSON: %v", err)
	}
	if envelope.Version != systemCredentialVersion || envelope.Credential.AccessToken != want.AccessToken {
		t.Fatalf("stored envelope = %#v", envelope)
	}
	got, err := store.Load(context.Background(), "account-a")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || got.ExpiresAt != want.ExpiresAt ||
		got.Scope != want.Scope || got.RequestedScope != want.RequestedScope || strings.Join(got.AuthorizedPaths, "\x00") != strings.Join(want.AuthorizedPaths, "\x00") ||
		got.RedirectURI != want.RedirectURI || string(got.Extra["custom"]) != "true" {
		t.Fatalf("loaded credential = %#v", got)
	}
	if err := store.Delete(context.Background(), "account-a"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !deleted {
		t.Error("backend Delete was not called")
	}
}

func TestSystemStoreMapsNotFoundAndBackendFailures(t *testing.T) {
	t.Parallel()

	backendFailure := errors.New("backend unavailable")
	tests := []struct {
		name string
		run  func(*SystemStore) error
		back *fakeKeyringBackend
		want error
	}{
		{
			name: "load not found",
			run:  func(s *SystemStore) error { _, err := s.Load(context.Background(), "key"); return err },
			back: &fakeKeyringBackend{get: func(string, string) (string, error) { return "", keyring.ErrNotFound }},
			want: ErrCredentialNotFound,
		},
		{
			name: "delete not found",
			run:  func(s *SystemStore) error { return s.Delete(context.Background(), "key") },
			back: &fakeKeyringBackend{delete: func(string, string) error { return keyring.ErrNotFound }},
			want: ErrCredentialNotFound,
		},
		{
			name: "load failure",
			run:  func(s *SystemStore) error { _, err := s.Load(context.Background(), "key"); return err },
			back: &fakeKeyringBackend{get: func(string, string) (string, error) { return "", backendFailure }},
			want: backendFailure,
		},
		{
			name: "save failure",
			run: func(s *SystemStore) error {
				return s.Save(context.Background(), "key", &Credential{AccessToken: "token"})
			},
			back: &fakeKeyringBackend{set: func(string, string, string) error { return backendFailure }},
			want: backendFailure,
		},
		{
			name: "delete failure",
			run:  func(s *SystemStore) error { return s.Delete(context.Background(), "key") },
			back: &fakeKeyringBackend{delete: func(string, string) error { return backendFailure }},
			want: backendFailure,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := &SystemStore{service: "service", backend: tc.back}
			if err := tc.run(store); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want errors.Is(..., %v)", err, tc.want)
			}
		})
	}
}

func TestSystemStoreRejectsInvalidPayloads(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		payload string
		want    string
	}{
		{name: "invalid JSON", payload: "not-json", want: "invalid credential"},
		{name: "unsupported version", payload: `{"version":2,"credential":{"access_token":"token"}}`, want: "unsupported version"},
		{name: "missing token", payload: `{"version":1,"credential":{"refresh_token":"refresh"}}`, want: "no access token"},
		{name: "oversized payload", payload: strings.Repeat("x", maxSystemCredentialSize+1), want: "2048-byte"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &SystemStore{
				service: "service",
				backend: &fakeKeyringBackend{get: func(string, string) (string, error) { return tc.payload, nil }},
			}
			_, err := store.Load(context.Background(), "key")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, tc.want)
			}
		})
	}

	store := &SystemStore{service: "service", backend: &fakeKeyringBackend{}}
	for _, tc := range []struct {
		name       string
		key        string
		credential *Credential
	}{
		{name: "empty key", key: "", credential: &Credential{AccessToken: "token"}},
		{name: "nil credential", key: "key"},
		{name: "empty access token", key: "key", credential: &Credential{AccessToken: " "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(context.Background(), tc.key, tc.credential); err == nil {
				t.Fatal("Save() unexpectedly succeeded")
			}
		})
	}
	invalidJSON := &Credential{
		AccessToken: "token",
		Extra:       map[string]json.RawMessage{"broken": json.RawMessage(`{`)},
	}
	if err := store.Save(context.Background(), "key", invalidJSON); err == nil || !strings.Contains(err.Error(), "encode credential") {
		t.Fatalf("Save(invalid JSON) error = %v", err)
	}
	backendCalled := false
	limitedStore := &SystemStore{
		service: "service",
		backend: &fakeKeyringBackend{set: func(string, string, string) error {
			backendCalled = true
			return nil
		}},
	}
	oversized := &Credential{
		AccessToken: "token",
		Extra: map[string]json.RawMessage{
			"large": json.RawMessage(`"` + strings.Repeat("x", maxSystemCredentialSize) + `"`),
		},
	}
	if err := limitedStore.Save(context.Background(), "key", oversized); err == nil || !strings.Contains(err.Error(), "2048-byte") {
		t.Fatalf("Save(oversized) error = %v", err)
	}
	if backendCalled {
		t.Error("oversized credential reached keyring backend")
	}
}

func TestSystemStoreHonorsContextBeforeAndAfterBackendCall(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	store := &SystemStore{
		service: "service",
		backend: &fakeKeyringBackend{
			get:    func(string, string) (string, error) { called = true; return "", nil },
			set:    func(string, string, string) error { called = true; return nil },
			delete: func(string, string) error { called = true; return nil },
		},
	}
	if _, err := store.Load(cancelled, "key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Load(cancelled) error = %v", err)
	}
	if err := store.Save(cancelled, "key", &Credential{AccessToken: "token"}); !errors.Is(err, context.Canceled) {
		t.Errorf("Save(cancelled) error = %v", err)
	}
	if err := store.Delete(cancelled, "key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Delete(cancelled) error = %v", err)
	}
	if called {
		t.Fatal("backend called despite context already being cancelled")
	}

	loadCtx, cancelLoad := context.WithCancel(context.Background())
	loadStore := &SystemStore{
		service: "service",
		backend: &fakeKeyringBackend{get: func(string, string) (string, error) {
			cancelLoad()
			return `{"version":1,"credential":{"access_token":"token"}}`, nil
		}},
	}
	if _, err := loadStore.Load(loadCtx, "key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Load(context cancelled by backend) error = %v", err)
	}

	saveCtx, cancelSave := context.WithCancel(context.Background())
	saveStore := &SystemStore{
		service: "service",
		backend: &fakeKeyringBackend{set: func(string, string, string) error {
			cancelSave()
			return nil
		}},
	}
	if err := saveStore.Save(saveCtx, "key", &Credential{AccessToken: "token"}); !errors.Is(err, context.Canceled) {
		t.Errorf("Save(context cancelled by backend) error = %v", err)
	}
}
