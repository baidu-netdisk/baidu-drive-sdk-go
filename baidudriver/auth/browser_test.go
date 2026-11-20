package auth

import (
	"context"
	"errors"
	"net"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func TestSystemBrowserCommand(t *testing.T) {
	t.Parallel()

	const authorizationURL = "https://pan.baidu.com/union/oauth/authorize?state=a&code_challenge=b"
	tests := []struct {
		goos     string
		wantName string
		wantArgs []string
	}{
		{goos: "darwin", wantName: "open", wantArgs: []string{authorizationURL}},
		{goos: "windows", wantName: "rundll32", wantArgs: []string{"url.dll,FileProtocolHandler", authorizationURL}},
		{goos: "linux", wantName: "xdg-open", wantArgs: []string{authorizationURL}},
		{goos: "freebsd", wantName: "xdg-open", wantArgs: []string{authorizationURL}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.goos, func(t *testing.T) {
			t.Parallel()
			name, args, err := systemBrowserCommand(tc.goos, authorizationURL)
			if err != nil {
				t.Fatalf("systemBrowserCommand() error = %v", err)
			}
			if name != tc.wantName || !reflect.DeepEqual(args, tc.wantArgs) {
				t.Fatalf("command = %q %#v, want %q %#v", name, args, tc.wantName, tc.wantArgs)
			}
		})
	}
	if _, _, err := systemBrowserCommand("plan9", authorizationURL); err == nil {
		t.Fatal("systemBrowserCommand() accepted an unsupported operating system")
	}
}

func TestBeginOptionalBrowserOpenAfterListenerStarts(t *testing.T) {
	redirectURI := unusedRedirectURI(t)
	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("browser unavailable")
	var openedURL string
	cfg := baseSessionConfig(redirectURI, NewMemoryStore())
	cfg.OpenBrowser = true
	cfg.BrowserOpener = func(ctx context.Context, authorizationURL string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		connection, err := net.DialTimeout("tcp", redirectURL.Host, time.Second)
		if err != nil {
			return err
		}
		_ = connection.Close()
		openedURL = authorizationURL
		return wantErr
	}

	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	t.Cleanup(func() { session.callback.close() })
	if openedURL == "" || openedURL != session.AuthorizationURL {
		t.Fatal("browser opener did not receive the generated authorization URL")
	}
	if !errors.Is(session.BrowserOpenError, wantErr) {
		t.Fatalf("BrowserOpenError = %v, want wrapped opener error", session.BrowserOpenError)
	}
}

func TestBeginDoesNotOpenBrowserForStoredCredential(t *testing.T) {
	store := NewMemoryStore()
	if err := store.Save(context.Background(), "test-account", &Credential{
		AccessToken: "cached-access",
		ExpiresAt:   time.Now().Add(time.Hour),
		Scope:       "basic netdisk",
	}); err != nil {
		t.Fatal(err)
	}
	cfg := baseSessionConfig(unusedRedirectURI(t), store)
	cfg.OpenBrowser = true
	called := false
	cfg.BrowserOpener = func(context.Context, string) error {
		called = true
		return nil
	}

	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	if session.AuthorizationURL != "" || session.BrowserOpenError != nil {
		t.Fatalf("stored session unexpectedly requires browser authorization: %#v", session)
	}
	if called {
		t.Fatal("browser opener was called for a stored credential")
	}
}
