package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreRoundTripUsesDeepCopies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewMemoryStore()
	original := &Credential{
		AccessToken:     "access-one",
		RefreshToken:    "refresh-one",
		ExpiresAt:       time.Now().Add(time.Hour),
		Scope:           "nd_open_read,nd_open_write",
		RequestedScope:  defaultScope,
		AuthorizedPaths: []string{"/apps/MyApp/images", "/apps/MyApp/docs"},
		RedirectURI:     "http://127.0.0.1:8787/oauth/callback?state=stored-state",
		Extra: map[string]json.RawMessage{
			"custom": json.RawMessage(`{"nested":true}`),
		},
	}
	if err := store.Save(ctx, "account", original); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	original.AccessToken = "mutated-after-save"
	original.AuthorizedPaths[0] = "/mutated-after-save"
	original.Extra["custom"][0] = 'x'

	loaded, err := store.Load(ctx, "account")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.AccessToken != "access-one" || loaded.AuthorizedPaths[0] != "/apps/MyApp/images" || string(loaded.Extra["custom"]) != `{"nested":true}` {
		t.Fatalf("stored credential was aliased: %#v", loaded)
	}
	loaded.AccessToken = "mutated-after-load"
	loaded.AuthorizedPaths[0] = "/mutated-after-load"
	loaded.Extra["custom"][0] = 'y'

	reloaded, err := store.Load(ctx, "account")
	if err != nil {
		t.Fatalf("second Load() error = %v", err)
	}
	if reloaded.AccessToken != "access-one" || reloaded.AuthorizedPaths[0] != "/apps/MyApp/images" || string(reloaded.Extra["custom"]) != `{"nested":true}` {
		t.Fatalf("loaded credential was aliased: %#v", reloaded)
	}
}

func TestMemoryStoreValidationContextAndDelete(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	ctx := context.Background()
	if _, err := store.Load(ctx, "missing"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Load(missing) error = %v", err)
	}
	if err := store.Delete(ctx, "missing"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Delete(missing) error = %v", err)
	}
	for _, tc := range []struct {
		name       string
		key        string
		credential *Credential
	}{
		{name: "empty key", key: " ", credential: &Credential{AccessToken: "token"}},
		{name: "nil credential", key: "key"},
		{name: "empty token", key: "key", credential: &Credential{AccessToken: " "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := store.Save(ctx, tc.key, tc.credential); err == nil {
				t.Fatal("Save() unexpectedly succeeded")
			}
		})
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.Load(cancelled, "key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Load(cancelled) error = %v", err)
	}
	if err := store.Save(cancelled, "key", &Credential{AccessToken: "token"}); !errors.Is(err, context.Canceled) {
		t.Errorf("Save(cancelled) error = %v", err)
	}
	if err := store.Delete(cancelled, "key"); !errors.Is(err, context.Canceled) {
		t.Errorf("Delete(cancelled) error = %v", err)
	}
	if _, err := store.Load(nil, "key"); err == nil {
		t.Error("Load(nil context) unexpectedly succeeded")
	}

	var zeroValue MemoryStore
	if err := zeroValue.Save(ctx, "saved", &Credential{AccessToken: "token"}); err != nil {
		t.Fatalf("zero-value MemoryStore.Save() error = %v", err)
	}
	if err := zeroValue.Delete(ctx, "saved"); err != nil {
		t.Fatalf("Delete(saved) error = %v", err)
	}
	if _, err := zeroValue.Load(ctx, "saved"); !errors.Is(err, ErrCredentialNotFound) {
		t.Fatalf("Load(deleted) error = %v", err)
	}
}

func TestMemoryStoreConcurrentAccess(t *testing.T) {
	t.Parallel()

	store := NewMemoryStore()
	ctx := context.Background()
	const workers = 32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := strings.Repeat("k", i%5+1)
			_ = store.Save(ctx, key, &Credential{AccessToken: key})
			_, _ = store.Load(ctx, key)
		}()
	}
	wg.Wait()
}

func TestCredentialStoreKeyIsStableAndSeparatesApplications(t *testing.T) {
	t.Parallel()

	first := credentialStoreKey("app", "http://127.0.0.1:8080/callback", "basic,netdisk")
	if got := credentialStoreKey("app", "http://127.0.0.1:8080/callback", "basic,netdisk"); got != first {
		t.Fatalf("credentialStoreKey is unstable: %q != %q", got, first)
	}
	if !strings.HasPrefix(first, "directory-oauth-v1-") || len(first) != len("directory-oauth-v1-")+64 {
		t.Fatalf("credentialStoreKey shape = %q", first)
	}
	if got := credentialStoreKey("other-app", "http://127.0.0.1:8080/callback", "basic,netdisk"); got == first {
		t.Error("different applications share a credential key")
	}
	if got := credentialStoreKey("app", "http://127.0.0.1:8081/callback", "basic,netdisk"); got == first {
		t.Error("different redirects share a credential key")
	}
	if got := credentialStoreKey("app", "http://127.0.0.1:8080/callback", "basic"); got == first {
		t.Error("different requested scopes share a credential key")
	}
}

func TestCredentialUsableScopeAndExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		credential *Credential
		scope      string
		skew       time.Duration
		want       bool
	}{
		{name: "nil", scope: defaultScope},
		{name: "missing access token", credential: &Credential{ExpiresAt: now.Add(time.Hour)}, scope: defaultScope},
		{name: "missing expiry", credential: &Credential{AccessToken: "token"}, scope: defaultScope},
		{name: "at skew boundary", credential: &Credential{AccessToken: "token", ExpiresAt: now.Add(5 * time.Minute)}, scope: defaultScope, skew: 5 * time.Minute},
		{name: "missing granted scope", credential: &Credential{AccessToken: "token", ExpiresAt: now.Add(time.Hour)}, scope: defaultScope, skew: 5 * time.Minute},
		{name: "no required scope", credential: &Credential{AccessToken: "token", ExpiresAt: now.Add(time.Hour)}, skew: 5 * time.Minute, want: true},
		{name: "valid normalized scope", credential: &Credential{AccessToken: "token", ExpiresAt: now.Add(time.Hour), Scope: "NETDISK basic extra"}, scope: "basic,netdisk", skew: 5 * time.Minute, want: true},
		{name: "missing required scope", credential: &Credential{AccessToken: "token", ExpiresAt: now.Add(time.Hour), Scope: "basic"}, scope: "basic,netdisk", skew: 5 * time.Minute},
		{name: "requested scope separates grant vocabulary", credential: &Credential{AccessToken: "token", ExpiresAt: now.Add(time.Hour), Scope: "nd_open_read,nd_open_write", RequestedScope: "basic,netdisk"}, scope: "basic,netdisk", skew: 5 * time.Minute, want: true},
		{name: "requested scope remains authoritative", credential: &Credential{AccessToken: "token", ExpiresAt: now.Add(time.Hour), Scope: "basic,netdisk", RequestedScope: "basic"}, scope: "basic,netdisk", skew: 5 * time.Minute},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := credentialUsable(tc.credential, tc.scope, tc.skew, now); got != tc.want {
				t.Errorf("credentialUsable() = %v, want %v", got, tc.want)
			}
		})
	}
}
