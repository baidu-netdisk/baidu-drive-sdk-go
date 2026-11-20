package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	diagnosticAppKey       = "diag-client-W4m8/+"
	diagnosticSecret       = "diag-secret-K9v3/+"
	diagnosticCode         = "diag-code-Q7p2/+"
	diagnosticAccessToken  = "diag-access-N5s1/+"
	diagnosticRefreshToken = "diag-refresh-J8d6/+"
	diagnosticRequestID    = "diag-request-500"
	diagnosticServerTime   = int64(1788165296)
	diagnosticLogID        = "diag-logid-500"
	diagnosticYLD          = "diag-yld-500"
	diagnosticNewno        = "diag-newno-500"
)

type synchronizedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *synchronizedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(data)
}

func (b *synchronizedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type diagnosticFailureTrace struct {
	log              string
	state            string
	authorizationURL string
	tokenRequestURL  string
	redirectURI      string
}

func runDiagnosticTokenFailure(t *testing.T, debug bool) diagnosticFailureTrace {
	t.Helper()

	requestURLs := make(chan string, 1)
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestURLs <- tokenServerURL(r)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Logid", diagnosticLogID)
		w.Header().Set("Yld", diagnosticYLD)
		w.Header().Set("X-Request-Id", diagnosticAccessToken)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"errno":500,"newno":"`+diagnosticNewno+`","show_msg":"all fail","request_id":"`+
			diagnosticRequestID+`","server_time":`+strconv.FormatInt(diagnosticServerTime, 10)+
			`,"access_token":"`+diagnosticAccessToken+`","refresh_token":"`+diagnosticRefreshToken+`"}`)
	}))
	defer tokenServer.Close()

	logger := &synchronizedBuffer{}
	redirectURI := unusedRedirectURI(t)
	cfg := Config{
		AppKey:        diagnosticAppKey,
		SecretKey:     diagnosticSecret,
		RedirectURI:   redirectURI,
		Store:         NewMemoryStore(),
		StoreKey:      "diagnostic-test-account",
		TokenEndpoint: tokenServer.URL + "/xpan/oauth/get",
		Debug:         debug,
		Logger:        logger,
	}
	session, err := Begin(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Begin() error = %v", err)
	}
	t.Cleanup(func() {
		if session.callback != nil {
			session.callback.close()
		}
	})

	authorizationURL, err := url.Parse(session.AuthorizationURL)
	if err != nil {
		t.Fatalf("url.Parse(AuthorizationURL): %v", err)
	}
	challenge := authorizationURL.Query().Get("code_challenge")
	if challenge != "" {
		t.Fatal("authorization URL must not carry a PKCE challenge")
	}
	callbackURL := session.RedirectURI + "?" + url.Values{
		"state": {session.state},
		"code":  {diagnosticCode},
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

	_, err = session.Wait(context.Background())
	var tokenErr *TokenError
	if !errors.As(err, &tokenErr) {
		t.Fatalf("Wait() error = %T %v, want *TokenError", err, err)
	}
	if tokenErr.StatusCode != http.StatusInternalServerError || tokenErr.Errno != 500 || tokenErr.RequestID != diagnosticRequestID {
		t.Fatalf("TokenError = %#v", tokenErr)
	}

	return diagnosticFailureTrace{
		log:              logger.String(),
		state:            session.state,
		authorizationURL: session.AuthorizationURL,
		tokenRequestURL:  tokenServer.URL + <-requestURLs,
		redirectURI:      redirectURI,
	}
}

func tokenServerURL(request *http.Request) string {
	return request.URL.RequestURI()
}

func TestDiagnosticsDisabledProducesNoOutput(t *testing.T) {
	trace := runDiagnosticTokenFailure(t, false)
	if trace.log != "" {
		t.Fatalf("Debug=false wrote %d diagnostic bytes", len(trace.log))
	}
}

func TestDiagnosticsToken500RecordsSafeEvidence(t *testing.T) {
	trace := runDiagnosticTokenFailure(t, true)
	logText := trace.log

	for _, event := range []string{
		"begin",
		"credential_store_load",
		"callback_listener_started",
		"authorization_ready",
		"callback_received",
		"callback_validated",
		"token_request_started",
		"token_response_received",
		"token_exchange_failed",
	} {
		requireDiagnosticContains(t, logText, event)
	}
	for name, value := range map[string]string{
		"http_status": "500",
		"errno":       "500",
		"request_id":  diagnosticRequestID,
		"server_time": strconv.FormatInt(diagnosticServerTime, 10),
		"logid":       diagnosticLogID,
		"yld":         diagnosticYLD,
		"newno":       diagnosticNewno,
	} {
		requireDiagnosticField(t, logText, name, value)
	}
	for _, evidence := range []string{
		"content_type", "application/json",
		"content_length", "body_bytes", "duration_ms",
		"query_keys", "method", "client_id", "client_secret", "code", "redirect_uri",
		"client_id_length", "code_length", "secret_present", "redirect_uri_hash",
	} {
		requireDiagnosticContains(t, logText, evidence)
	}
	requireDiagnosticLength(t, logText, "client_id_length", len(diagnosticAppKey))
	requireDiagnosticLength(t, logText, "code_length", len(diagnosticCode))

	for name, value := range map[string]string{
		"AppKey":             diagnosticAppKey,
		"SecretKey":          diagnosticSecret,
		"authorization code": diagnosticCode,
		"state":              trace.state,
		"access token":       diagnosticAccessToken,
		"refresh token":      diagnosticRefreshToken,
		"redirect URI":       trace.redirectURI,
		"authorization URL":  trace.authorizationURL,
		"token request URL":  trace.tokenRequestURL,
	} {
		if value != "" && strings.Contains(logText, value) {
			t.Errorf("diagnostics expose %s", name)
		}
	}
}

func requireDiagnosticContains(t *testing.T, logText, value string) {
	t.Helper()
	if !strings.Contains(strings.ToLower(logText), strings.ToLower(value)) {
		t.Errorf("diagnostics do not contain %q", value)
	}
}

func requireDiagnosticLength(t *testing.T, logText, name string, length int) {
	t.Helper()
	pattern := `(?i)["']?` + regexp.QuoteMeta(name) + `["']?\s*[:=]\s*` + strconv.Itoa(length) + `(?:\D|$)`
	if !regexp.MustCompile(pattern).MatchString(logText) {
		t.Errorf("diagnostics do not contain %s=%d", name, length)
	}
}

func requireDiagnosticField(t *testing.T, logText, name, value string) {
	t.Helper()
	pattern := `(?i)["']?` + regexp.QuoteMeta(name) + `["']?\s*[:=]\s*["']?` + regexp.QuoteMeta(value) + `(?:["'\s,}]|$)`
	if !regexp.MustCompile(pattern).MatchString(logText) {
		t.Errorf("diagnostics do not contain %s=%q", name, value)
	}
}
