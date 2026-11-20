package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

const (
	maxCallbackQueryLength = 8 * 1024
	maxAuthorizationCode   = 4 * 1024
	maxOAuthErrorCode      = 256
	maxOAuthErrorText      = 1024
)

var (
	// ErrSessionAlreadyUsed is returned when Wait is called more than once.
	ErrSessionAlreadyUsed = errors.New("baidupan auth: authorization session has already been used")
)

// AuthorizationError reports a denial returned to the loopback callback.
type AuthorizationError struct {
	// Errno is the numeric service error for a token-mode failure callback.
	Errno       int
	Code        string
	Description string
}

func (e *AuthorizationError) Error() string {
	if e.Description == "" {
		return "baidupan auth: authorization denied: " + e.Code
	}
	return "baidupan auth: authorization denied: " + e.Code + ": " + e.Description
}

// Result contains the authenticated API client and a copy of the credential
// that was saved. Credential values must not be logged.
type Result struct {
	Client     *api.Client
	Credential Credential
	FromStore  bool
}

// Session represents one authorization attempt. AuthorizationURL is empty
// when a valid credential was loaded from the configured store.
type Session struct {
	AuthorizationURL string
	RedirectURI      string
	// BrowserOpenError is non-nil only when Config.OpenBrowser was requested
	// and the external browser process could not be started. AuthorizationURL
	// remains valid for manual use.
	BrowserOpenError error

	config    normalizedConfig
	state     string
	cached    *Credential
	callback  *callbackServer
	waited    atomic.Bool
	startedAt time.Time
}

// Begin is the first developer-facing call. It loads a reusable credential or
// starts the loopback listener before returning the directory authorization
// URL, avoiding a browser-to-listener race.
func Begin(ctx context.Context, cfg Config) (*Session, error) {
	beginStarted := time.Now()
	normalized, err := normalizeConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	diagnostics := normalized.diagnostics
	diagnostics.log("begin", "outcome=started client_id_hash=%s callback_host=%q callback_path=%q authorization_host=%q authorization_path=%q token_host=%q token_path=%q scope=%q",
		diagnosticHash(normalized.appKey),
		diagnosticSafeText(normalized.redirectURL.Host, 512, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.redirectURL.EscapedPath(), 1024, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.authorizationEndpoint.Host, 512, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.authorizationEndpoint.EscapedPath(), 1024, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.tokenEndpoint.Host, 512, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.tokenEndpoint.EscapedPath(), 1024, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.scope, 1024, normalized.appKey, normalized.secretKey))

	storeStarted := time.Now()
	credential, err := normalized.store.Load(ctx, normalized.storeKey)
	if err == nil && normalized.credentialUsable(credential, time.Now()) {
		diagnostics.log("credential_store_load", "outcome=hit usable=true store_type=%q store_key_hash=%s duration_ms=%d",
			fmt.Sprintf("%T", normalized.store), diagnosticHash(normalized.storeKey), diagnosticDurationMS(storeStarted))
		return &Session{
			RedirectURI: normalized.redirectURI,
			config:      normalized,
			cached:      cloneCredential(credential),
			startedAt:   beginStarted,
		}, nil
	}
	if err == nil {
		diagnostics.log("credential_store_load", "outcome=hit usable=false store_type=%q store_key_hash=%s duration_ms=%d",
			fmt.Sprintf("%T", normalized.store), diagnosticHash(normalized.storeKey), diagnosticDurationMS(storeStarted))
	} else if errors.Is(err, ErrCredentialNotFound) {
		diagnostics.log("credential_store_load", "outcome=miss store_type=%q store_key_hash=%s duration_ms=%d",
			fmt.Sprintf("%T", normalized.store), diagnosticHash(normalized.storeKey), diagnosticDurationMS(storeStarted))
	}
	if err != nil && !errors.Is(err, ErrCredentialNotFound) {
		diagnostics.log("credential_store_load", "outcome=failed error_class=store_load store_type=%q store_key_hash=%s duration_ms=%d",
			fmt.Sprintf("%T", normalized.store), diagnosticHash(normalized.storeKey), diagnosticDurationMS(storeStarted))
		return nil, fmt.Errorf("baidupan auth: load stored credential: %w", err)
	}

	state, err := randomURLSafe(32)
	if err != nil {
		return nil, fmt.Errorf("baidupan auth: generate state: %w", err)
	}
	callback, err := startCallbackServer(ctx, normalized, state, beginStarted)
	if err != nil {
		diagnostics.log("callback_listener_started", "outcome=failed error_class=listen callback_host=%q callback_path=%q duration_ms=%d",
			diagnosticSafeText(normalized.redirectURL.Host, 512, normalized.appKey, normalized.secretKey),
			diagnosticSafeText(normalized.redirectURL.EscapedPath(), 1024, normalized.appKey, normalized.secretKey), diagnosticDurationMS(beginStarted))
		return nil, err
	}
	diagnostics.log("callback_listener_started", "outcome=succeeded callback_host=%q callback_path=%q duration_ms=%d",
		diagnosticSafeText(normalized.redirectURL.Host, 512, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.redirectURL.EscapedPath(), 1024, normalized.appKey, normalized.secretKey), diagnosticDurationMS(beginStarted))

	authorizationURL := buildAuthorizationURL(normalized, state)
	diagnostics.log("authorization_ready", "outcome=succeeded endpoint_host=%q endpoint_path=%q query_keys=%q client_id_length=%d redirect_uri_hash=%s state_length=%d",
		diagnosticSafeText(normalized.authorizationEndpoint.Host, 512, normalized.appKey, normalized.secretKey),
		diagnosticSafeText(normalized.authorizationEndpoint.EscapedPath(), 1024, normalized.appKey, normalized.secretKey),
		"authorization_type,client_id,redirect_uri,response_type,scope,state",
		len(normalized.appKey), diagnosticHash(normalized.redirectURI), len(state))
	session := &Session{
		AuthorizationURL: authorizationURL,
		RedirectURI:      normalized.redirectURI,
		config:           normalized,
		state:            state,
		callback:         callback,
		startedAt:        beginStarted,
	}
	if normalized.openBrowser {
		opener := normalized.browserOpener
		if opener == nil {
			opener = openSystemBrowser
		}
		browserStarted := time.Now()
		if openErr := opener(ctx, authorizationURL); openErr != nil {
			session.BrowserOpenError = fmt.Errorf("baidupan auth: open authorization URL in browser: %w", openErr)
			diagnostics.log("browser_open", "outcome=failed error_class=browser_launcher duration_ms=%d", diagnosticDurationMS(browserStarted))
		} else {
			diagnostics.log("browser_open", "outcome=succeeded duration_ms=%d", diagnosticDurationMS(browserStarted))
		}
	} else {
		diagnostics.log("browser_open", "outcome=skipped reason=disabled")
	}
	return session, nil
}

// Wait is the second developer-facing call. It waits for one valid callback,
// receives tokens (or exchanges a code), atomically stores the credential, and
// returns an API client. Token-mode browser success is sent only after Save.
func (s *Session) Wait(ctx context.Context) (_ *Result, resultErr error) {
	if s == nil {
		return nil, errors.New("baidupan auth: session must not be nil")
	}
	if ctx == nil {
		return nil, errors.New("baidupan auth: context must not be nil")
	}
	if !s.waited.CompareAndSwap(false, true) {
		s.config.diagnostics.log("session_completed", "outcome=failed error_class=session_reused duration_ms=%d", diagnosticDurationMS(s.startedAt))
		return nil, ErrSessionAlreadyUsed
	}
	if s.cached != nil {
		if err := ctx.Err(); err != nil {
			s.config.diagnostics.log("session_completed", "outcome=failed source=store error_class=context duration_ms=%d", diagnosticDurationMS(s.startedAt))
			return nil, err
		}
		s.config.diagnostics.log("session_completed", "outcome=succeeded source=store duration_ms=%d", diagnosticDurationMS(s.startedAt))
		return s.newResult(s.cached, true), nil
	}
	if s.callback == nil {
		return nil, errors.New("baidupan auth: session has no callback server")
	}
	defer s.callback.close()

	var callback callbackResult
	select {
	case callback = <-s.callback.results:
		if callback.outcome == nil {
			s.callback.close()
		} else {
			defer func() {
				callback.outcome <- resultErr
				<-callback.responded
			}()
		}
	case serveErr := <-s.callback.serveErrors:
		s.config.diagnostics.log("session_completed", "outcome=failed error_class=callback_server duration_ms=%d", diagnosticDurationMS(s.startedAt))
		return nil, fmt.Errorf("baidupan auth: callback server: %w", serveErr)
	case <-ctx.Done():
		s.config.diagnostics.log("session_completed", "outcome=failed error_class=context duration_ms=%d", diagnosticDurationMS(s.startedAt))
		return nil, fmt.Errorf("baidupan auth: wait for callback: %w", ctx.Err())
	}
	if callback.err != nil {
		s.config.diagnostics.log("session_completed", "outcome=failed error_class=authorization_denied duration_ms=%d", diagnosticDurationMS(s.startedAt))
		return nil, callback.err
	}

	credential := callback.credential
	if credential == nil {
		var err error
		credential, err = exchangeAuthorizationCode(ctx, s.config, callback.code, s.state)
		if err != nil {
			s.config.diagnostics.log("session_completed", "outcome=failed error_class=token_exchange duration_ms=%d", diagnosticDurationMS(s.startedAt))
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	storeStarted := time.Now()
	if err := s.config.store.Save(ctx, s.config.storeKey, credential); err != nil {
		s.config.diagnostics.log("credential_store_save", "outcome=failed error_class=store_save store_type=%q store_key_hash=%s duration_ms=%d",
			fmt.Sprintf("%T", s.config.store), diagnosticHash(s.config.storeKey), diagnosticDurationMS(storeStarted))
		return nil, fmt.Errorf("baidupan auth: persist credential: %w", err)
	}
	s.config.diagnostics.log("credential_store_save", "outcome=succeeded store_type=%q store_key_hash=%s duration_ms=%d",
		fmt.Sprintf("%T", s.config.store), diagnosticHash(s.config.storeKey), diagnosticDurationMS(storeStarted))
	s.config.diagnostics.log("session_completed", "outcome=succeeded source=authorization duration_ms=%d", diagnosticDurationMS(s.startedAt))
	return s.newResult(credential, false), nil
}

func (s *Session) newResult(credential *Credential, fromStore bool) *Result {
	copy := cloneCredential(credential)
	options := append([]api.Option(nil), s.config.clientOptions...)
	options = append(options, api.WithAccessToken(copy.AccessToken))
	return &Result{
		Client:     api.NewClient(options...),
		Credential: *copy,
		FromStore:  fromStore,
	}
}

func buildAuthorizationURL(cfg normalizedConfig, state string) string {
	u := *cfg.authorizationEndpoint
	query := u.Query()
	query.Set("authorization_type", "directory")
	query.Set("client_id", cfg.appKey)
	query.Set("redirect_uri", cfg.redirectURI)
	query.Set("response_type", "code")
	query.Set("scope", cfg.scope)
	query.Set("state", state)
	u.RawQuery = query.Encode()
	return u.String()
}

func randomURLSafe(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

func credentialUsable(credential *Credential, requiredScope string, skew time.Duration, now time.Time) bool {
	if credential == nil || strings.TrimSpace(credential.AccessToken) == "" || credential.ExpiresAt.IsZero() {
		return false
	}
	if !credential.ExpiresAt.After(now.Add(skew)) {
		return false
	}
	required := scopeSet(requiredScope)
	if len(required) == 0 {
		return true
	}
	grantedScope := credential.RequestedScope
	if strings.TrimSpace(grantedScope) == "" {
		grantedScope = credential.Scope
	}
	if strings.TrimSpace(grantedScope) == "" {
		return false
	}
	granted := scopeSet(grantedScope)
	for scope := range required {
		if !granted[scope] {
			return false
		}
	}
	return true
}

func (cfg normalizedConfig) credentialUsable(credential *Credential, now time.Time) bool {
	if credential == nil {
		return false
	}
	return credentialUsable(credential, cfg.scope, cfg.expirySkew, now)
}

func scopeSet(scope string) map[string]bool {
	values := strings.FieldsFunc(strings.ToLower(scope), func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	})
	result := make(map[string]bool, len(values))
	for _, value := range values {
		if value != "" {
			result[value] = true
		}
	}
	return result
}

type callbackResult struct {
	code       string
	err        error
	credential *Credential
	// Token handlers wait for durable storage before replying to the browser.
	outcome   chan error
	responded chan struct{}
}

type callbackServer struct {
	context     context.Context
	server      *http.Server
	listener    net.Listener
	results     chan callbackResult
	serveErrors chan error
	done        chan struct{}
	consumed    atomic.Bool
	closeOnce   sync.Once
	diagnostics *diagnosticLogger
}

func startCallbackServer(ctx context.Context, cfg normalizedConfig, expectedState string, startedAt time.Time) (*callbackServer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", cfg.listenAddress)
	if err != nil {
		return nil, fmt.Errorf("baidupan auth: listen on %s: %w", cfg.listenAddress, err)
	}
	callback := &callbackServer{
		context:     ctx,
		listener:    listener,
		results:     make(chan callbackResult, 1),
		serveErrors: make(chan error, 1),
		done:        make(chan struct{}),
		diagnostics: cfg.diagnostics,
	}
	callbackHandler := callback.handler(cfg.redirectURL, expectedState)
	writeTimeout := 5 * time.Second
	expectedPath := cfg.redirectURL.EscapedPath()
	callback.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pathMatches := r.URL.EscapedPath() == expectedPath
			callback.diagnostics.log("callback_received", "method_is_get=%t path_matches=%t query_bytes=%d", r.Method == http.MethodGet, pathMatches, len(r.URL.RawQuery))
			if !pathMatches {
				setCallbackHeaders(w)
				callback.diagnostics.log("callback_rejected", "outcome=rejected reason=path_mismatch http_status=%d", http.StatusNotFound)
				http.NotFound(w, r)
				return
			}
			callbackHandler(w, r)
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       10 * time.Second,
		MaxHeaderBytes:    8 * 1024,
	}
	go func() {
		if serveErr := callback.server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			select {
			case callback.serveErrors <- serveErr:
			default:
			}
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			select {
			case callback.serveErrors <- ctx.Err():
			default:
			}
			callback.close()
		case <-callback.done:
		}
	}()
	return callback, nil
}

func (s *callbackServer) handler(redirectURL *url.URL, expectedState string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCallbackHeaders(w)
		reject := func(status int, reason, responseText string) {
			s.diagnostics.log("callback_rejected", "outcome=rejected reason=%s http_status=%d", reason, status)
			http.Error(w, responseText, status)
		}
		if r.Method != http.MethodGet {
			reject(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !strings.EqualFold(r.Host, redirectURL.Host) {
			reject(http.StatusBadRequest, "host_mismatch", "invalid callback host")
			return
		}
		if len(r.URL.RawQuery) > maxCallbackQueryLength {
			reject(http.StatusRequestURITooLong, "query_too_large", "callback query is too large")
			return
		}
		query, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			reject(http.StatusBadRequest, "query_parse", "invalid OAuth callback query")
			return
		}
		for key, maxLength := range map[string]int{
			"state":             1024,
			"code":              maxAuthorizationCode,
			"error":             maxOAuthErrorCode,
			"error_description": maxOAuthErrorText,
		} {
			if values, exists := query[key]; exists {
				if len(values) != 1 {
					reject(http.StatusBadRequest, "duplicate_parameter", "duplicate OAuth callback parameter")
					return
				}
				value := strings.TrimSpace(values[0])
				if value == "" || len(value) > maxLength {
					reject(http.StatusBadRequest, "invalid_parameter", "invalid OAuth callback parameter")
					return
				}
			}
		}
		state, ok := singleQueryValue(query, "state", 1, 1024)
		if !ok || subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
			reject(http.StatusBadRequest, "state_mismatch", "invalid OAuth state")
			return
		}
		if s.consumed.Load() {
			reject(http.StatusConflict, "already_consumed", "callback already consumed")
			return
		}

		oauthError, hasError := optionalSingleQueryValue(query, "error", maxOAuthErrorCode)
		code, hasCode := optionalSingleQueryValue(query, "code", maxAuthorizationCode)
		if hasError == hasCode {
			reject(http.StatusBadRequest, "code_error_cardinality", "callback must contain exactly one code or error")
			return
		}
		if !s.consumed.CompareAndSwap(false, true) {
			reject(http.StatusConflict, "already_consumed", "callback already consumed")
			return
		}
		if hasError {
			s.diagnostics.log("callback_validated", "outcome=authorization_denied state_valid=true code_present=false oauth_error_present=true")
			description, _ := optionalSingleQueryValue(query, "error_description", maxOAuthErrorText)
			s.results <- callbackResult{err: &AuthorizationError{
				Code:        safeText(strings.ReplaceAll(oauthError, expectedState, "***"), maxOAuthErrorCode),
				Description: safeText(strings.ReplaceAll(description, expectedState, "***"), maxOAuthErrorText),
			}}
			_, _ = fmt.Fprintln(w, "Authorization was not granted. You may close this window.")
			return
		}

		s.diagnostics.log("callback_validated", "outcome=accepted state_valid=true code_present=true oauth_error_present=false code_length=%d", len(code))
		s.results <- callbackResult{code: code}
		_, _ = fmt.Fprintln(w, "Authorization received. You may close this window.")
	}
}

func (s *callbackServer) close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.done != nil {
			close(s.done)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.server.Shutdown(ctx)
		_ = s.listener.Close()
	})
}

func singleQueryValue(query url.Values, key string, minLength, maxLength int) (string, bool) {
	values, ok := query[key]
	if !ok || len(values) != 1 {
		return "", false
	}
	value := strings.TrimSpace(values[0])
	return value, len(value) >= minLength && len(value) <= maxLength
}

func optionalSingleQueryValue(query url.Values, key string, maxLength int) (string, bool) {
	values, ok := query[key]
	if !ok {
		return "", false
	}
	if len(values) != 1 {
		return "", false
	}
	value := strings.TrimSpace(values[0])
	if value == "" || len(value) > maxLength {
		return "", false
	}
	return value, true
}

func setCallbackHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func safeText(value string, maxLength int) string {
	value = strings.Join(strings.Fields(value), " ")
	runes := []rune(value)
	if len(runes) > maxLength {
		value = string(runes[:maxLength])
	}
	return value
}
