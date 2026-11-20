package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestSessionExchangesStateAndParsesNestedTokenData(t *testing.T) {
	const (
		code                = "nested-contract-code-Q7p2/+"
		accessToken         = "xpan-nested-access-N5s1/+"
		refreshToken        = "xpan-nested-refresh-J8d6/+"
		responseScope       = "nd_open_read,nd_open_write"
		requestID           = "743402410528110592"
		serverTime    int64 = 1786675938
	)
	wantPaths := []string{"/apps/MyApp/private-images-Q7p2", "/apps/MyApp/private-documents-J8d6"}

	redirectURI := unusedRedirectURI(t)
	logger := &synchronizedBuffer{}
	tokenRequests := make(chan url.Values, 1)
	returnedRedirectURIs := make(chan string, 1)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		tokenRequests <- query
		returnedRedirectURI := redirectURI + "?" + url.Values{"state": {query.Get("state")}}.Encode()
		returnedRedirectURIs <- returnedRedirectURI

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"errno":       0,
			"newno":       "",
			"show_msg":    "",
			"request_id":  json.Number(requestID),
			"server_time": serverTime,
			"data": map[string]any{
				"access_token":     accessToken,
				"expires_in":       2592000,
				"refresh_token":    refreshToken,
				"scope":            responseScope,
				"authorized_paths": wantPaths,
				"redirect_uri":     returnedRedirectURI,
			},
		}); err != nil {
			t.Errorf("encode token response: %v", err)
		}
	}))
	defer tokenServer.Close()

	store := NewMemoryStore()
	cfg := baseSessionConfig(redirectURI, store)
	cfg.TokenEndpoint = tokenServer.URL + "/xpan/oauth/get"
	cfg.Debug = true
	cfg.Logger = logger
	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	t.Cleanup(func() {
		if session.callback != nil {
			session.callback.close()
		}
	})

	callbackURL := session.RedirectURI + "?" + url.Values{
		"state": {session.state},
		"code":  {code},
	}.Encode()
	response, err := http.Get(callbackURL)
	if err != nil {
		t.Fatalf("OAuth callback request error = %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("OAuth callback status = %d", response.StatusCode)
	}

	result, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	tokenQuery := <-tokenRequests
	if got := tokenQuery.Get("state"); got != session.state {
		t.Errorf("token state = %q, want session state", got)
	}
	if values, ok := tokenQuery["state"]; !ok || len(values) != 1 {
		t.Errorf("token state values = %#v, want exactly one", values)
	}
	if tokenQuery.Get("code") != code || tokenQuery.Has("code_verifier") {
		t.Errorf("token query does not preserve the callback code, or leaks a PKCE verifier")
	}

	returnedRedirectURI := <-returnedRedirectURIs
	wantCredential := Credential{
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		ExpiresIn:       2592000,
		Scope:           responseScope,
		RequestedScope:  defaultScope,
		AuthorizedPaths: wantPaths,
		RedirectURI:     returnedRedirectURI,
		RequestID:       requestID,
		ServerTime:      serverTime,
	}
	assertNestedTokenCredential(t, result.Credential, wantCredential)
	if result.FromStore || result.Client == nil {
		t.Fatalf("result = %#v, want a newly authorized API client", result)
	}

	stored, err := store.Load(context.Background(), "test-account")
	if err != nil {
		t.Fatalf("Load(saved credential) error = %v", err)
	}
	assertNestedTokenCredential(t, *stored, wantCredential)

	logText := logger.String()
	for _, event := range []string{"token_request_started", "token_response_received", "token_exchange_succeeded"} {
		if !strings.Contains(logText, "event="+event) {
			t.Errorf("diagnostics do not contain event=%s", event)
		}
	}
	for name, secret := range map[string]string{
		"authorization code": code,
		"session state":      session.state,
		"access token":       accessToken,
		"refresh token":      refreshToken,
		"authorized path 1":  wantPaths[0],
		"authorized path 2":  wantPaths[1],
		"response redirect":  returnedRedirectURI,
	} {
		if strings.Contains(logText, secret) {
			t.Errorf("diagnostics expose nested response %s", name)
		}
	}
}

func assertNestedTokenCredential(t *testing.T, got, want Credential) {
	t.Helper()
	if got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || got.ExpiresIn != want.ExpiresIn {
		t.Errorf("token credential fields = access:%t refresh:%t expires_in:%d, want access:true refresh:true expires_in:%d",
			got.AccessToken == want.AccessToken, got.RefreshToken == want.RefreshToken, got.ExpiresIn, want.ExpiresIn)
	}
	if got.Scope != want.Scope || got.RequestedScope != want.RequestedScope {
		t.Errorf("credential scopes = response:%q requested:%q, want response:%q requested:%q",
			got.Scope, got.RequestedScope, want.Scope, want.RequestedScope)
	}
	if !reflect.DeepEqual(got.AuthorizedPaths, want.AuthorizedPaths) {
		t.Errorf("AuthorizedPaths = %#v, want %#v", got.AuthorizedPaths, want.AuthorizedPaths)
	}
	if got.RedirectURI != want.RedirectURI {
		t.Errorf("RedirectURI = %q, want %q", got.RedirectURI, want.RedirectURI)
	}
	if got.RequestID != want.RequestID || got.ServerTime != want.ServerTime {
		t.Errorf("response metadata = request_id:%q server_time:%d, want request_id:%q server_time:%d",
			got.RequestID, got.ServerTime, want.RequestID, want.ServerTime)
	}
	if got.ExpiresAt.IsZero() || got.CreatedAt.IsZero() {
		t.Error("credential timestamps were not populated")
	}
}
