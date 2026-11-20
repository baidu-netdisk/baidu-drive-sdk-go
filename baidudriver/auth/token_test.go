package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type observedTokenRequest struct {
	method   string
	path     string
	rawQuery string
	accept   string
	agent    string
}

type failingTokenBody struct {
	err    error
	closed atomic.Bool
}

func (b *failingTokenBody) Read([]byte) (int, error) { return 0, b.err }

func (b *failingTokenBody) Close() error {
	b.closed.Store(true)
	return nil
}

func tokenConfigForServer(t *testing.T, server *httptest.Server) normalizedConfig {
	t.Helper()
	endpoint, err := url.Parse(server.URL + "/xpan/oauth/get")
	if err != nil {
		t.Fatalf("url.Parse(): %v", err)
	}
	return normalizedConfig{
		appKey:        "app key/+",
		secretKey:     "secret key/+",
		redirectURI:   "http://127.0.0.1:18800/oauth/callback",
		scope:         defaultScope,
		tokenEndpoint: endpoint,
		httpClient:    server.Client(),
	}
}

func TestExchangeAuthorizationCodeExactGETQueryAndSuccess(t *testing.T) {
	t.Parallel()

	requests := make(chan observedTokenRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- observedTokenRequest{
			method:   r.Method,
			path:     r.URL.Path,
			rawQuery: r.URL.RawQuery,
			accept:   r.Header.Get("Accept"),
			agent:    r.Header.Get("User-Agent"),
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"access_token":"access-value",
			"refresh_token":"refresh-value",
			"expires_in":3600,
			"scope":"basic,netdisk",
			"token_type":"Bearer",
			"session_key":"session-key",
			"session_secret":"session-secret",
			"request_id":92233720368547758071234,
			"server_time":1788192000,
			"custom_field":{"enabled":true}
		}`)
	}))
	defer server.Close()

	cfg := tokenConfigForServer(t, server)
	const code = "code value/+"
	const state = "state value/+"
	before := time.Now().UTC()
	credential, err := exchangeAuthorizationCode(context.Background(), cfg, code, state)
	after := time.Now().UTC()
	if err != nil {
		t.Fatalf("exchangeAuthorizationCode() error = %v", err)
	}
	request := <-requests
	if request.method != http.MethodGet || request.path != "/xpan/oauth/get" {
		t.Errorf("request = %s %s", request.method, request.path)
	}
	wantQuery := url.Values{
		"client_id":     {cfg.appKey},
		"client_secret": {cfg.secretKey},
		"code":          {code},
		"method":        {"token"},
		"redirect_uri":  {cfg.redirectURI},
		"state":         {state},
	}.Encode()
	if request.rawQuery != wantQuery {
		t.Errorf("RawQuery = %q, want %q", request.rawQuery, wantQuery)
	}
	if strings.Contains(request.rawQuery, "grant_type") {
		t.Error("xpan token request must not carry grant_type; the endpoint keys off method=token")
	}
	if strings.Contains(request.rawQuery, "code_verifier") || strings.Contains(request.rawQuery, "code_challenge") {
		t.Error("token request must not carry PKCE parameters; this contract rejects them")
	}
	if request.accept != "application/json" || request.agent != "BaiduPanGoSDK/1.0" {
		t.Errorf("headers Accept=%q User-Agent=%q", request.accept, request.agent)
	}
	if credential.AccessToken != "access-value" || credential.RefreshToken != "refresh-value" || credential.ExpiresIn != 3600 {
		t.Fatalf("credential = %#v", credential)
	}
	if credential.RequestID != "92233720368547758071234" {
		t.Errorf("RequestID = %q", credential.RequestID)
	}
	if credential.ExpiresAt.Before(before.Add(time.Hour)) || credential.ExpiresAt.After(after.Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v, want between %v and %v", credential.ExpiresAt, before.Add(time.Hour), after.Add(time.Hour))
	}
	if got := string(credential.Extra["custom_field"]); got != `{"enabled":true}` {
		t.Errorf("Extra custom_field = %q", got)
	}
	for _, known := range []string{"access_token", "refresh_token", "expires_in", "request_id", "server_time"} {
		if _, ok := credential.Extra[known]; ok {
			t.Errorf("known field %q leaked into Extra", known)
		}
	}
}

func TestExchangeAuthorizationCodeNestedDataIsAuthoritative(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		wantAccess string
		wantMsg    string
	}{
		{
			name:       "nested token wins over flat compatibility fields",
			body:       `{"access_token":"flat-access","expires_in":3600,"data":{"access_token":"nested-access","refresh_token":"nested-refresh","expires_in":7200,"scope":"nd_open_read,nd_open_write","redirect_uri":"http://127.0.0.1:18800/oauth/callback?state=state-value"}}`,
			wantAccess: "nested-access",
		},
		{
			name:    "explicit null does not fall back to flat token",
			body:    `{"access_token":"flat-access","expires_in":3600,"data":null}`,
			wantMsg: "response contains no access token",
		},
		{
			name:    "empty nested object does not fall back to flat token",
			body:    `{"access_token":"flat-access","expires_in":3600,"data":{}}`,
			wantMsg: "response contains no access token",
		},
		{
			name:    "invalid nested shape is rejected",
			body:    `{"access_token":"flat-access","expires_in":3600,"data":[]}`,
			wantMsg: "invalid token data response",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()

			credential, err := exchangeAuthorizationCode(context.Background(), tokenConfigForServer(t, server), "code", "state-value")
			if tc.wantMsg != "" {
				var tokenErr *TokenError
				if !errors.As(err, &tokenErr) || !strings.Contains(tokenErr.Message, tc.wantMsg) {
					t.Fatalf("error = %T %v, want *TokenError containing %q", err, err, tc.wantMsg)
				}
				return
			}
			if err != nil {
				t.Fatalf("exchangeAuthorizationCode() error = %v", err)
			}
			if credential.AccessToken != tc.wantAccess || credential.RefreshToken != "nested-refresh" || credential.ExpiresIn != 7200 {
				t.Fatalf("credential = %#v", credential)
			}
			if credential.Extra != nil {
				t.Fatalf("nested data leaked into Extra: %#v", credential.Extra)
			}
		})
	}
}

func TestExchangeAuthorizationCodeRedactsBothResponseShapes(t *testing.T) {
	t.Parallel()

	secrets := []string{
		"flat-access-secret",
		"flat-refresh-secret",
		"/apps/flat-secret",
		"http://127.0.0.1:18800/oauth/callback?state=flat-secret-state",
		"nested-access-secret",
		"nested-refresh-secret",
		"/apps/nested-secret",
		"http://127.0.0.1:18800/oauth/callback?state=nested-secret-state",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Logid", strings.Join(secrets, " "))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errno":            -1,
			"show_msg":         strings.Join(secrets, " "),
			"request_id":       secrets[1],
			"access_token":     secrets[0],
			"refresh_token":    secrets[1],
			"expires_in":       3600,
			"authorized_paths": []string{secrets[2]},
			"redirect_uri":     secrets[3],
			"data": map[string]any{
				"access_token":     secrets[4],
				"refresh_token":    secrets[5],
				"expires_in":       3600,
				"authorized_paths": []string{secrets[6]},
				"redirect_uri":     secrets[7],
			},
		})
	}))
	defer server.Close()

	logger := &synchronizedBuffer{}
	cfg := tokenConfigForServer(t, server)
	cfg.diagnostics = newDiagnosticLogger(true, logger)
	_, err := exchangeAuthorizationCode(context.Background(), cfg, "code", "state-value")
	if err == nil {
		t.Fatal("exchangeAuthorizationCode() unexpectedly succeeded")
	}
	combined := err.Error() + "\n" + logger.String()
	for _, secret := range secrets {
		if strings.Contains(combined, secret) {
			t.Errorf("error or diagnostics expose mixed response secret %q", secret)
		}
	}
}

func TestTokenRedirectURIValidation(t *testing.T) {
	t.Parallel()

	const configured = "http://127.0.0.1:18800/oauth/callback"
	const state = "state-value"
	for _, tc := range []struct {
		name string
		raw  string
		want bool
	}{
		{name: "valid", raw: configured + "?state=" + state, want: true},
		{name: "additional response metadata", raw: configured + "?granted=1&state=" + state, want: true},
		{name: "missing state", raw: configured},
		{name: "wrong state", raw: configured + "?state=other"},
		{name: "duplicate state", raw: configured + "?state=" + state + "&state=" + state},
		{name: "wrong callback", raw: "http://127.0.0.1:18800/other?state=" + state},
		{name: "fragment", raw: configured + "?state=" + state + "#fragment"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := validTokenRedirectURI(tc.raw, configured, state); got != tc.want {
				t.Errorf("validTokenRedirectURI() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestExtraTokenFieldsDropsKnownKeysCaseInsensitively(t *testing.T) {
	t.Parallel()

	extra := extraTokenFields([]byte(`{"ACCESS_TOKEN":"secret","Authorized_Paths":["/secret"],"DaTa":{"refresh_token":"secret"},"Custom":{"enabled":true}}`))
	if len(extra) != 1 || string(extra["Custom"]) != `{"enabled":true}` {
		t.Fatalf("extraTokenFields() = %#v", extra)
	}
}

func TestExchangeAuthorizationCodeXpanErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		body       string
		wantStatus int
		wantErrno  int
		wantMsg    string
		wantID     string
	}{
		{
			name:       "errno envelope on HTTP 200",
			status:     http.StatusOK,
			body:       `{"errno":-6,"show_msg":"authorization expired","request_id":12345678901234567890}`,
			wantStatus: http.StatusOK,
			wantErrno:  -6,
			wantMsg:    "authorization expired",
			wantID:     "12345678901234567890",
		},
		{
			name:       "OAuth envelope on HTTP 400",
			status:     http.StatusBadRequest,
			body:       `{"error":"invalid_grant","error_description":"code rejected","request_id":"request-text"}`,
			wantStatus: http.StatusBadRequest,
			wantMsg:    "code rejected",
			wantID:     "request-text",
		},
		{
			name:       "HTTP status fallback",
			status:     http.StatusBadGateway,
			body:       `{}`,
			wantStatus: http.StatusBadGateway,
			wantMsg:    "Bad Gateway",
		},
		{
			name:       "invalid JSON",
			status:     http.StatusOK,
			body:       `not-json`,
			wantStatus: http.StatusOK,
			wantMsg:    "invalid JSON response",
		},
		{
			name:       "missing access token",
			status:     http.StatusOK,
			body:       `{"expires_in":3600,"request_id":"missing-token"}`,
			wantStatus: http.StatusOK,
			wantMsg:    "response contains no access token",
			wantID:     "missing-token",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()

			_, err := exchangeAuthorizationCode(context.Background(), tokenConfigForServer(t, server), "authorization-code-value", "state-value")
			var tokenErr *TokenError
			if !errors.As(err, &tokenErr) {
				t.Fatalf("error = %T %v, want *TokenError", err, err)
			}
			if tokenErr.StatusCode != tc.wantStatus || tokenErr.Errno != tc.wantErrno || tokenErr.RequestID != tc.wantID {
				t.Errorf("TokenError = %#v", tokenErr)
			}
			if !strings.Contains(tokenErr.Message, tc.wantMsg) {
				t.Errorf("Message = %q, want containing %q", tokenErr.Message, tc.wantMsg)
			}
		})
	}
}

func TestExchangeAuthorizationCodeRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, strings.Repeat("x", maxTokenResponseSize+1))
	}))
	defer server.Close()
	_, err := exchangeAuthorizationCode(context.Background(), tokenConfigForServer(t, server), "code", "state")
	if err == nil || !strings.Contains(err.Error(), "exceeds 1 MiB") {
		t.Fatalf("error = %v", err)
	}
}

func TestExchangeAuthorizationCodeRejectsInvalidLifetime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    string
		wantMsg string
	}{
		{name: "zero lifetime", body: `{"access_token":"token","expires_in":0,"scope":"basic,netdisk"}`, wantMsg: "invalid expires_in"},
		{name: "negative lifetime", body: `{"access_token":"token","expires_in":-1,"scope":"basic,netdisk"}`, wantMsg: "invalid expires_in"},
		{name: "overflow lifetime", body: fmt.Sprintf(`{"access_token":"token","expires_in":%d,"scope":"basic,netdisk"}`, maxCredentialLifetimeSeconds+1), wantMsg: "invalid expires_in"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			_, err := exchangeAuthorizationCode(context.Background(), tokenConfigForServer(t, server), "code", "state")
			var tokenErr *TokenError
			if !errors.As(err, &tokenErr) || !strings.Contains(tokenErr.Message, tc.wantMsg) {
				t.Fatalf("error = %T %v, want *TokenError containing %q", err, err, tc.wantMsg)
			}
		})
	}
}

func TestExchangeAuthorizationCodeValidatesInputsAndCancellation(t *testing.T) {
	t.Parallel()

	endpoint, _ := url.Parse("https://pan.baidu.com/xpan/oauth/get")
	cfg := normalizedConfig{tokenEndpoint: endpoint, httpClient: http.DefaultClient}
	if _, err := exchangeAuthorizationCode(nil, cfg, "code", "state"); err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("nil context error = %v", err)
	}
	for _, tc := range []struct{ code, state string }{
		{"", "state"},
		{"code", ""},
		{" ", "state"},
		{"code", " "},
	} {
		if _, err := exchangeAuthorizationCode(context.Background(), cfg, tc.code, tc.state); err == nil {
			t.Errorf("exchangeAuthorizationCode(%q, %q) unexpectedly succeeded", tc.code, tc.state)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := exchangeAuthorizationCode(cancelled, cfg, "code", "state"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request error = %v", err)
	}
}

func TestExchangeAuthorizationCodeRequestAndReadFailures(t *testing.T) {
	t.Parallel()

	endpoint, _ := url.Parse("https://pan.baidu.com/xpan/oauth/get")
	networkErr := errors.New("network failed")
	networkCfg := normalizedConfig{
		tokenEndpoint: endpoint,
		httpClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, networkErr
		})},
	}
	if _, err := exchangeAuthorizationCode(context.Background(), networkCfg, "code", "state"); !errors.Is(err, networkErr) {
		t.Fatalf("network error = %v", err)
	}

	readErr := errors.New("read failed")
	body := &failingTokenBody{err: readErr}
	readCfg := normalizedConfig{
		tokenEndpoint: endpoint,
		httpClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: body, Header: make(http.Header), Request: request}, nil
		})},
	}
	if _, err := exchangeAuthorizationCode(context.Background(), readCfg, "code", "state"); !errors.Is(err, readErr) {
		t.Fatalf("read error = %v", err)
	}
	if !body.closed.Load() {
		t.Error("token response body was not closed after read failure")
	}
}

func TestExchangeAuthorizationCodeCancellationDuringRequest(t *testing.T) {
	t.Parallel()

	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := exchangeAuthorizationCode(ctx, tokenConfigForServer(t, server), "code", "state")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline exceeded", err)
	}
	select {
	case <-entered:
	default:
		t.Fatal("token request never reached the server")
	}
}

func TestExchangeAuthorizationCodeDoesNotFollowRedirect(t *testing.T) {
	t.Parallel()

	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected.Add(1)
		_, _ = io.WriteString(w, `{"access_token":"leaked"}`)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, target.URL, http.StatusFound)
	}))
	defer source.Close()

	cfg, normalizeErr := normalizeConfig(context.Background(), Config{
		AppKey:          "app key/+",
		SecretKey:       "secret key/+",
		RedirectURI:     "http://127.0.0.1:18800/oauth/callback",
		Store:           NewMemoryStore(),
		TokenEndpoint:   source.URL + "/xpan/oauth/get",
		OAuthHTTPClient: source.Client(),
	})
	if normalizeErr != nil {
		t.Fatalf("normalizeConfig() error = %v", normalizeErr)
	}
	_, err := exchangeAuthorizationCode(context.Background(), cfg, "sensitive-code", "sensitive-state")
	if err == nil {
		t.Fatal("token exchange followed redirect and treated redirected response as success")
	}
	if got := redirected.Load(); got != 0 {
		t.Fatalf("redirect target received %d requests; token query secrets may have leaked", got)
	}
}

func TestTokenErrorRedactsAllSensitiveValues(t *testing.T) {
	t.Parallel()

	const code = "sensitive-code"
	const state = "sensitive-state"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"errno":-1,"show_msg":%q,"request_id":%q}`, "bad secret key/+ "+code+" "+state, code)
	}))
	defer server.Close()
	_, err := exchangeAuthorizationCode(context.Background(), tokenConfigForServer(t, server), code, state)
	if err == nil {
		t.Fatal("exchangeAuthorizationCode() unexpectedly succeeded")
	}
	for _, secret := range []string{"secret key/+", code, state} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error exposes %q: %v", secret, err)
		}
	}
}

func TestTokenTransportErrorRedactsEncodedQuerySecrets(t *testing.T) {
	t.Parallel()

	endpoint, _ := url.Parse("https://pan.baidu.com/xpan/oauth/get")
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("failed request %s", request.URL.String())
	})
	cfg := normalizedConfig{
		appKey:        "app",
		secretKey:     "secret key/+",
		redirectURI:   "http://127.0.0.1:18800/oauth/callback",
		tokenEndpoint: endpoint,
		httpClient:    &http.Client{Transport: transport},
	}
	const code = "code value/+"
	const state = "state value/+"
	_, err := exchangeAuthorizationCode(context.Background(), cfg, code, state)
	if err == nil {
		t.Fatal("exchangeAuthorizationCode() unexpectedly succeeded")
	}
	for _, secret := range []string{cfg.secretKey, code, state, url.QueryEscape(cfg.secretKey), url.QueryEscape(code), url.QueryEscape(state)} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("transport error exposes %q: %v", secret, err)
		}
	}
	for cause := errors.Unwrap(err); cause != nil; cause = errors.Unwrap(cause) {
		for _, secret := range []string{cfg.appKey, cfg.secretKey, code, state, cfg.redirectURI} {
			if strings.Contains(cause.Error(), secret) {
				t.Fatalf("transport error cause exposes OAuth request data")
			}
		}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		t.Fatal("transport error chain exposes the original URL containing OAuth query credentials")
	}
}

func TestRawJSONText(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		raw  string
		want string
	}{
		{raw: `null`, want: ""},
		{raw: `"request-id"`, want: "request-id"},
		{raw: `12345678901234567890`, want: "12345678901234567890"},
	} {
		if got := rawJSONText([]byte(tc.raw)); got != tc.want {
			t.Errorf("rawJSONText(%s) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
