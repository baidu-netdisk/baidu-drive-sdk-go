package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/auth"
)

const liveKeyringService = "com.baidu.netdisk.openapi-go.live-test"

func TestLiveDirectoryOAuth(t *testing.T) {
	if os.Getenv("BAIDU_AUTH_LIVE") != "1" {
		t.Skip("set BAIDU_AUTH_LIVE=1 to run the live directory OAuth test")
	}

	appKey := strings.TrimSpace(os.Getenv("BAIDU_APP_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("BAIDU_SECRET_KEY"))
	redirectURI := strings.TrimSpace(os.Getenv("BAIDU_REDIRECT_URI"))
	for _, required := range []struct {
		name  string
		value string
	}{
		{name: "BAIDU_APP_KEY", value: appKey},
		{name: "BAIDU_SECRET_KEY", value: secretKey},
		{name: "BAIDU_REDIRECT_URI", value: redirectURI},
	} {
		if required.value == "" {
			t.Fatalf("%s must be set when BAIDU_AUTH_LIVE=1", required.name)
		}
	}

	store, err := auth.NewSystemStore(liveKeyringService)
	if err != nil {
		t.Fatalf("create live-test system store: %v", err)
	}
	storeKey := liveCredentialStoreKey(appKey, redirectURI)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		if err := store.Delete(cleanupCtx, storeKey); err != nil && !errors.Is(err, auth.ErrCredentialNotFound) {
			t.Errorf("delete live-test credential during cleanup: %v", err)
		}
	})
	if err := store.Delete(ctx, storeKey); err != nil && !errors.Is(err, auth.ErrCredentialNotFound) {
		t.Fatalf("delete stale live-test credential: %v", err)
	}

	cfg := auth.Config{
		AppKey:      appKey,
		SecretKey:   secretKey,
		RedirectURI: redirectURI,
		Store:       store,
		StoreKey:    storeKey,
		OpenBrowser: true,
		Debug:       true,
		Logger:      os.Stderr,
	}
	session, err := auth.Begin(ctx, cfg)
	if err != nil {
		t.Fatalf("begin live directory OAuth: %v", err)
	}
	if session.AuthorizationURL == "" {
		t.Fatal("first Begin returned an empty authorization URL after credential cleanup")
	}
	assertLiveAuthorizationURL(t, session.AuthorizationURL, appKey, redirectURI)
	if session.BrowserOpenError != nil {
		t.Logf("Automatic browser launch failed; open the URL manually: %v", session.BrowserOpenError)
		t.Logf("Authorization URL (manual fallback): %s", session.AuthorizationURL)
	} else {
		t.Log("Authorization URL was sent to the system browser")
	}

	result, err := session.Wait(ctx)
	if err != nil {
		t.Fatalf("wait for live directory OAuth: %v", err)
	}
	if result == nil || result.Client == nil {
		t.Fatal("live directory OAuth returned no authenticated client")
	}
	if result.FromStore {
		t.Fatal("first authorization unexpectedly reused a stored credential")
	}
	if strings.TrimSpace(result.Credential.AccessToken) == "" {
		t.Fatal("live directory OAuth returned an empty access token")
	}
	if result.Credential.ExpiresAt.IsZero() || !result.Credential.ExpiresAt.After(time.Now()) {
		t.Fatal("live directory OAuth returned a missing or expired ExpiresAt")
	}

	quota, err := result.Client.Nas.Quota(ctx, nil)
	if err != nil {
		t.Fatalf("validate authenticated client with quota API: %v", err)
	}
	if quota == nil {
		t.Fatal("quota API returned a nil response")
	}
	if quota.Total < quota.Used {
		t.Fatalf("quota invariant failed: total bytes are less than used bytes")
	}

	cachedSession, err := auth.Begin(ctx, cfg)
	if err != nil {
		t.Fatalf("begin with stored live-test credential: %v", err)
	}
	if cachedSession.AuthorizationURL != "" {
		t.Fatal("cached Begin returned a non-empty authorization URL")
	}
	cachedResult, err := cachedSession.Wait(ctx)
	if err != nil {
		t.Fatalf("load stored live-test credential: %v", err)
	}
	if cachedResult == nil || !cachedResult.FromStore {
		t.Fatal("second authorization did not report FromStore")
	}
}

func assertLiveAuthorizationURL(t *testing.T, rawURL, appKey, redirectURI string) {
	t.Helper()

	authorizationURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal("Begin returned an invalid authorization URL")
	}
	if authorizationURL.Scheme != "https" || authorizationURL.Host != "pan.baidu.com" || authorizationURL.EscapedPath() != "/union/oauth/authorize" {
		t.Fatal("Begin returned an unexpected authorization endpoint")
	}

	query := authorizationURL.Query()
	want := map[string]string{
		"authorization_type": "directory",
		"client_id":          appKey,
		"redirect_uri":       redirectURI,
		"response_type":      "code",
		"scope":              "basic,netdisk",
	}
	for key, value := range want {
		values := query[key]
		if len(values) != 1 || values[0] != value {
			t.Fatalf("authorization URL parameter %q is missing or incorrect", key)
		}
	}
	for _, key := range []string{"state"} {
		values := query[key]
		if len(values) != 1 || len(values[0]) != 43 {
			t.Fatalf("authorization URL parameter %q is missing or malformed", key)
		}
	}
	if len(query) != len(want)+1 {
		t.Fatal("authorization URL contains unexpected parameters")
	}
}

func liveCredentialStoreKey(appKey, redirectURI string) string {
	sum := sha256.Sum256([]byte(appKey + "\x00" + redirectURI))
	return "directory-oauth-live-test-" + hex.EncodeToString(sum[:])
}
