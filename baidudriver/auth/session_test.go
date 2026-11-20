package auth

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

type scriptedStore struct {
	load   func(context.Context, string) (*Credential, error)
	save   func(context.Context, string, *Credential) error
	delete func(context.Context, string) error
}

func (s *scriptedStore) Load(ctx context.Context, key string) (*Credential, error) {
	if s.load == nil {
		return nil, ErrCredentialNotFound
	}
	return s.load(ctx, key)
}

func (s *scriptedStore) Save(ctx context.Context, key string, credential *Credential) error {
	if s.save == nil {
		return nil
	}
	return s.save(ctx, key, credential)
}

func (s *scriptedStore) Delete(ctx context.Context, key string) error {
	if s.delete == nil {
		return nil
	}
	return s.delete(ctx, key)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func reserveLoopbackAddress(t *testing.T) (net.Listener, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen(): %v", err)
	}
	return listener, listener.Addr().String()
}

func unusedRedirectURI(t *testing.T) string {
	t.Helper()
	listener, address := reserveLoopbackAddress(t)
	if err := listener.Close(); err != nil {
		t.Fatalf("listener.Close(): %v", err)
	}
	return "http://" + address + "/oauth/callback"
}

func baseSessionConfig(redirectURI string, store CredentialStore) Config {
	return Config{
		AppKey:      "app-key",
		SecretKey:   "secret-key",
		RedirectURI: redirectURI,
		Store:       store,
		StoreKey:    "test-account",
	}
}

func callbackFixture(t *testing.T, expectedState string) (*callbackServer, http.HandlerFunc, *url.URL) {
	t.Helper()
	redirectURL, err := url.Parse("http://127.0.0.1:18801/oauth/callback")
	if err != nil {
		t.Fatalf("url.Parse(): %v", err)
	}
	server := &callbackServer{results: make(chan callbackResult, 1)}
	return server, server.handler(redirectURL, expectedState), redirectURL
}

func runCallback(handler http.Handler, redirectURL *url.URL, method, rawQuery, host string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, redirectURL.String()+"?"+rawQuery, nil)
	request.Host = host
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestCallbackHandlerHappyPathAndReplay(t *testing.T) {
	t.Parallel()

	const state = "expected-state"
	server, handler, redirectURL := callbackFixture(t, state)
	query := url.Values{"state": {state}, "code": {"authorization-code"}}.Encode()
	response := runCallback(handler, redirectURL, http.MethodGet, query, redirectURL.Host)
	if response.Code != http.StatusOK {
		t.Fatalf("callback status = %d, body = %q", response.Code, response.Body.String())
	}
	for key, want := range map[string]string{
		"Cache-Control":           "no-store",
		"Content-Security-Policy": "default-src 'none'; frame-ancestors 'none'",
		"Content-Type":            "text/plain; charset=utf-8",
		"Referrer-Policy":         "no-referrer",
		"X-Content-Type-Options":  "nosniff",
	} {
		if got := response.Header().Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	result := <-server.results
	if result.code != "authorization-code" || result.err != nil {
		t.Fatalf("callback result = %#v", result)
	}
	replay := runCallback(handler, redirectURL, http.MethodGet, query, redirectURL.Host)
	if replay.Code != http.StatusConflict {
		t.Fatalf("replay status = %d, want %d", replay.Code, http.StatusConflict)
	}
	select {
	case extra := <-server.results:
		t.Fatalf("replay emitted an extra result: %#v", extra)
	default:
	}
}

func TestCallbackHandlerInvalidRequestDoesNotConsumeSession(t *testing.T) {
	t.Parallel()

	const state = "expected-state"
	tests := []struct {
		name       string
		method     string
		rawQuery   string
		host       string
		wantStatus int
	}{
		{name: "wrong method", method: http.MethodPost, rawQuery: "state=expected-state&code=code", host: "127.0.0.1:18801", wantStatus: http.StatusMethodNotAllowed},
		{name: "wrong host", method: http.MethodGet, rawQuery: "state=expected-state&code=code", host: "localhost:18801", wantStatus: http.StatusBadRequest},
		{name: "wrong state", method: http.MethodGet, rawQuery: "state=wrong&code=code", host: "127.0.0.1:18801", wantStatus: http.StatusBadRequest},
		{name: "duplicate state", method: http.MethodGet, rawQuery: "state=expected-state&state=expected-state&code=code", host: "127.0.0.1:18801", wantStatus: http.StatusBadRequest},
		{name: "duplicate code", method: http.MethodGet, rawQuery: "state=expected-state&code=one&code=two", host: "127.0.0.1:18801", wantStatus: http.StatusBadRequest},
		{name: "code and error", method: http.MethodGet, rawQuery: "state=expected-state&code=code&error=denied", host: "127.0.0.1:18801", wantStatus: http.StatusBadRequest},
		{name: "neither code nor error", method: http.MethodGet, rawQuery: "state=expected-state", host: "127.0.0.1:18801", wantStatus: http.StatusBadRequest},
		{name: "malformed query", method: http.MethodGet, rawQuery: "state=expected-state&code=code&bad=%zz", host: "127.0.0.1:18801", wantStatus: http.StatusBadRequest},
		{name: "oversized query", method: http.MethodGet, rawQuery: "state=expected-state&code=code&padding=" + strings.Repeat("x", maxCallbackQueryLength), host: "127.0.0.1:18801", wantStatus: http.StatusRequestURITooLong},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server, handler, redirectURL := callbackFixture(t, state)
			response := runCallback(handler, redirectURL, tc.method, tc.rawQuery, tc.host)
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, tc.wantStatus, response.Body.String())
			}
			if server.consumed.Load() {
				t.Fatal("invalid callback consumed the session")
			}
			valid := runCallback(handler, redirectURL, http.MethodGet, "state=expected-state&code=valid", redirectURL.Host)
			if valid.Code != http.StatusOK {
				t.Fatalf("valid callback after invalid one = %d", valid.Code)
			}
			if result := <-server.results; result.code != "valid" {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestCallbackHandlerAuthorizationError(t *testing.T) {
	t.Parallel()

	const state = "secret-state"
	server, handler, redirectURL := callbackFixture(t, state)
	query := url.Values{
		"state":             {state},
		"error":             {"access_denied"},
		"error_description": {"user denied " + state + "\ntry again"},
	}.Encode()
	response := runCallback(handler, redirectURL, http.MethodGet, query, redirectURL.Host)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
	result := <-server.results
	var authorizationErr *AuthorizationError
	if !errors.As(result.err, &authorizationErr) {
		t.Fatalf("error = %T %v, want *AuthorizationError", result.err, result.err)
	}
	if authorizationErr.Code != "access_denied" || authorizationErr.Description != "user denied *** try again" {
		t.Fatalf("AuthorizationError = %#v", authorizationErr)
	}
	if strings.Contains(authorizationErr.Error(), state) {
		t.Errorf("AuthorizationError exposes state: %v", authorizationErr)
	}
}

func TestCallbackHandlerConcurrentReplayAllowsExactlyOne(t *testing.T) {
	t.Parallel()

	server, handler, redirectURL := callbackFixture(t, "state")
	query := "state=state&code=code"
	start := make(chan struct{})
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			statuses <- runCallback(handler, redirectURL, http.MethodGet, query, redirectURL.Host).Code
		}()
	}
	close(start)
	wg.Wait()
	close(statuses)
	counts := map[int]int{}
	for status := range statuses {
		counts[status]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusConflict] != 1 {
		t.Fatalf("statuses = %#v", counts)
	}
	if result := <-server.results; result.code != "code" {
		t.Fatalf("result = %#v", result)
	}
}

func TestBeginBuildsOAuth21ReadyURLAfterListenerStarts(t *testing.T) {
	store := NewMemoryStore()
	session, err := Begin(context.Background(), baseSessionConfig(unusedRedirectURI(t), store))
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	t.Cleanup(func() { session.callback.close() })

	authorizationURL, err := url.Parse(session.AuthorizationURL)
	if err != nil {
		t.Fatalf("url.Parse(AuthorizationURL): %v", err)
	}
	query := authorizationURL.Query()
	if query.Get("state") != session.state || len(session.state) != 43 {
		t.Errorf("state query/session = %q/%q", query.Get("state"), session.state)
	}
	if query.Has("code_challenge") || query.Has("code_challenge_method") {
		t.Error("authorization URL must not carry PKCE parameters; the xpan contract rejects them")
	}
	if strings.Contains(session.AuthorizationURL, "secret-key") {
		t.Error("AuthorizationURL contains SecretKey")
	}
	wrongPath := strings.Replace(session.RedirectURI, "/oauth/callback", "/oauth/other", 1)
	wrongPathResponse, err := http.Get(wrongPath + "?state=" + url.QueryEscape(session.state) + "&code=ignored")
	if err != nil {
		t.Fatalf("wrong-path callback request: %v", err)
	}
	_ = wrongPathResponse.Body.Close()
	if wrongPathResponse.StatusCode != http.StatusNotFound {
		t.Errorf("wrong-path callback status = %d, want %d", wrongPathResponse.StatusCode, http.StatusNotFound)
	}
	response, err := http.Get(session.RedirectURI + "?state=wrong&code=ignored")
	if err != nil {
		t.Fatalf("callback listener was not ready when Begin returned: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid-state callback status = %d", response.StatusCode)
	}
}

func TestBeginContextCancellationStopsCallbackServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	session, err := Begin(ctx, baseSessionConfig(unusedRedirectURI(t), NewMemoryStore()))
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	cancel()
	_, err = session.Wait(context.Background())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait() after Begin context cancellation error = %v", err)
	}
}

func TestSessionEndToEndPersistsCredential(t *testing.T) {
	store := NewMemoryStore()
	tokenRequests := make(chan url.Values, 1)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests <- r.URL.Query()
		_, _ = io.WriteString(w, `{"access_token":"new-access","refresh_token":"new-refresh","expires_in":3600,"scope":"basic,netdisk"}`)
	}))
	defer tokenServer.Close()

	cfg := baseSessionConfig(unusedRedirectURI(t), store)
	cfg.TokenEndpoint = tokenServer.URL + "/xpan/oauth/get"
	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	callbackQuery := url.Values{"state": {session.state}, "code": {"callback-code"}}.Encode()
	response, err := http.Get(session.RedirectURI + "?" + callbackQuery)
	if err != nil {
		t.Fatalf("callback request error = %v", err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("callback status = %d", response.StatusCode)
	}

	result, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if result.FromStore || result.Client == nil || result.Credential.AccessToken != "new-access" {
		t.Fatalf("result = %#v", result)
	}
	tokenQuery := <-tokenRequests
	if tokenQuery.Get("code") != "callback-code" || tokenQuery.Has("code_verifier") {
		t.Errorf("token query = %#v", tokenQuery)
	}
	stored, err := store.Load(context.Background(), "test-account")
	if err != nil {
		t.Fatalf("Load(saved credential) error = %v", err)
	}
	if stored.AccessToken != "new-access" || stored.RefreshToken != "new-refresh" || stored.ExpiresAt.IsZero() {
		t.Fatalf("stored credential = %#v", stored)
	}
	if _, err := session.Wait(context.Background()); !errors.Is(err, ErrSessionAlreadyUsed) {
		t.Fatalf("second Wait() error = %v", err)
	}
}

func TestBeginReusesUsableStoredCredentialWithoutBinding(t *testing.T) {
	listener, address := reserveLoopbackAddress(t)
	defer listener.Close()
	store := NewMemoryStore()
	want := &Credential{
		AccessToken:  "cached-access",
		RefreshToken: "cached-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		Scope:        "basic netdisk",
	}
	if err := store.Save(context.Background(), "test-account", want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	cfg := baseSessionConfig("http://"+address+"/oauth/callback", store)
	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() should not bind for a cached credential: %v", err)
	}
	if session.AuthorizationURL != "" || session.callback != nil {
		t.Fatalf("cached session unexpectedly requires authorization: %#v", session)
	}
	result, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	if !result.FromStore || result.Credential.AccessToken != want.AccessToken {
		t.Fatalf("result = %#v", result)
	}
}

func TestBeginDoesNotHideStoreLoadFailure(t *testing.T) {
	listener, address := reserveLoopbackAddress(t)
	defer listener.Close()
	wantErr := errors.New("keychain unavailable")
	store := &scriptedStore{load: func(context.Context, string) (*Credential, error) { return nil, wantErr }}
	_, err := Begin(context.Background(), baseSessionConfig("http://"+address+"/oauth/callback", store))
	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "load stored credential") {
		t.Fatalf("Begin() error = %v", err)
	}
}

func TestSessionReturnsStoreSaveFailure(t *testing.T) {
	wantErr := errors.New("keychain write failed")
	store := &scriptedStore{save: func(context.Context, string, *Credential) error { return wantErr }}
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"access","expires_in":3600,"scope":"basic,netdisk"}`)
	}))
	defer tokenServer.Close()
	cfg := baseSessionConfig(unusedRedirectURI(t), store)
	cfg.TokenEndpoint = tokenServer.URL
	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	response, err := http.Get(session.RedirectURI + "?" + url.Values{"state": {session.state}, "code": {"code"}}.Encode())
	if err != nil {
		t.Fatalf("callback request error = %v", err)
	}
	_ = response.Body.Close()
	_, err = session.Wait(context.Background())
	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "persist credential") {
		t.Fatalf("Wait() error = %v", err)
	}
}

func TestSessionWaitTimeoutClosesListener(t *testing.T) {
	redirectURI := unusedRedirectURI(t)
	session, err := Begin(context.Background(), baseSessionConfig(redirectURI, NewMemoryStore()))
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = session.Wait(waitCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait() error = %v", err)
	}
	u, _ := url.Parse(redirectURI)
	listener, listenErr := net.Listen("tcp", u.Host)
	if listenErr != nil {
		t.Fatalf("callback listener was not released after timeout: %v", listenErr)
	}
	_ = listener.Close()
	if _, err := session.Wait(context.Background()); !errors.Is(err, ErrSessionAlreadyUsed) {
		t.Fatalf("second Wait() error = %v", err)
	}
}

func TestSessionCachedWaitHonorsContextAndNilReceiver(t *testing.T) {
	t.Parallel()

	var nilSession *Session
	if _, err := nilSession.Wait(context.Background()); err == nil {
		t.Fatal("nil Session.Wait unexpectedly succeeded")
	}
	session := &Session{cached: &Credential{AccessToken: "token"}, config: normalizedConfig{}}
	if _, err := session.Wait(nil); err == nil {
		t.Fatal("Wait(nil context) unexpectedly succeeded")
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := session.Wait(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait(cancelled) error = %v", err)
	}
}

func TestResultClientInjectsCredentialAccessToken(t *testing.T) {
	var observedURL *url.URL
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		copyURL := *request.URL
		observedURL = &copyURL
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Request:    request,
		}, nil
	})}
	store := NewMemoryStore()
	credential := &Credential{AccessToken: "stored-access", ExpiresAt: time.Now().Add(time.Hour), Scope: defaultScope}
	if err := store.Save(context.Background(), "test-account", credential); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	cfg := baseSessionConfig("http://127.0.0.1:1/oauth/callback", store)
	cfg.ClientOptions = []api.Option{api.WithHTTPClient(httpClient), api.WithAccessToken("wrong-option-token")}
	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	result, err := session.Wait(context.Background())
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}
	request, _ := http.NewRequest(http.MethodGet, "http://example.invalid/resource?access_token=stale", nil)
	var body map[string]any
	if _, err := result.Client.Do(context.Background(), request, &body); err != nil {
		t.Fatalf("Client.Do() error = %v", err)
	}
	if observedURL == nil || observedURL.Query().Get("access_token") != "stored-access" {
		t.Fatalf("observed URL = %v", observedURL)
	}
	if request.URL.Query().Get("access_token") != "stale" {
		t.Errorf("client transport mutated the caller's request URL: %s", request.URL)
	}
}
