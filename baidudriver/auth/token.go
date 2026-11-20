package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxTokenResponseSize = 1024 * 1024

const maxCredentialLifetimeSeconds int64 = 100 * 366 * 24 * 60 * 60

// TokenError reports a structured failure from the directory token endpoint.
type TokenError struct {
	StatusCode int
	Errno      int
	Message    string
	RequestID  string
}

type tokenRequestError struct {
	cause   error
	message string
}

func (e *tokenRequestError) Error() string {
	return "baidupan auth: token request: " + e.message
}

func (e *tokenRequestError) Unwrap() error {
	return e.cause
}

func (e *TokenError) Error() string {
	parts := []string{"baidupan auth: token exchange failed"}
	if e.StatusCode != 0 {
		parts = append(parts, "status="+strconv.Itoa(e.StatusCode))
	}
	if e.Errno != 0 {
		parts = append(parts, "errno="+strconv.Itoa(e.Errno))
	}
	if e.Message != "" {
		parts = append(parts, "msg="+e.Message)
	}
	if e.RequestID != "" {
		parts = append(parts, "request_id="+e.RequestID)
	}
	return strings.Join(parts, " ")
}

type tokenEnvelope struct {
	AccessToken      string          `json:"access_token"`
	RefreshToken     string          `json:"refresh_token"`
	ExpiresIn        int64           `json:"expires_in"`
	Scope            string          `json:"scope"`
	TokenType        string          `json:"token_type"`
	SessionKey       string          `json:"session_key"`
	SessionSecret    string          `json:"session_secret"`
	AuthorizedPaths  []string        `json:"authorized_paths"`
	RedirectURI      string          `json:"redirect_uri"`
	Data             json.RawMessage `json:"data"`
	Errno            int             `json:"errno"`
	Newno            json.RawMessage `json:"newno"`
	ShowMsg          string          `json:"show_msg"`
	Errmsg           string          `json:"errmsg"`
	ErrorMsg         string          `json:"error_msg"`
	OAuthError       string          `json:"error"`
	ErrorDescription string          `json:"error_description"`
	RequestID        json.RawMessage `json:"request_id"`
	ServerTime       int64           `json:"server_time"`
}

type tokenData struct {
	AccessToken     string   `json:"access_token"`
	RefreshToken    string   `json:"refresh_token"`
	ExpiresIn       int64    `json:"expires_in"`
	Scope           string   `json:"scope"`
	TokenType       string   `json:"token_type"`
	SessionKey      string   `json:"session_key"`
	SessionSecret   string   `json:"session_secret"`
	AuthorizedPaths []string `json:"authorized_paths"`
	RedirectURI     string   `json:"redirect_uri"`
}

func (e tokenEnvelope) payload() (tokenData, error) {
	if len(e.Data) != 0 {
		var payload tokenData
		if err := json.Unmarshal(e.Data, &payload); err != nil {
			return tokenData{}, err
		}
		return payload, nil
	}
	return e.flatPayload(), nil
}

func (e tokenEnvelope) flatPayload() tokenData {
	return tokenData{
		AccessToken:     e.AccessToken,
		RefreshToken:    e.RefreshToken,
		ExpiresIn:       e.ExpiresIn,
		Scope:           e.Scope,
		TokenType:       e.TokenType,
		SessionKey:      e.SessionKey,
		SessionSecret:   e.SessionSecret,
		AuthorizedPaths: e.AuthorizedPaths,
		RedirectURI:     e.RedirectURI,
	}
}

func exchangeAuthorizationCode(ctx context.Context, cfg normalizedConfig, code, state string) (*Credential, error) {
	if ctx == nil {
		return nil, errors.New("baidupan auth: context must not be nil")
	}
	if strings.TrimSpace(code) == "" || strings.TrimSpace(state) == "" {
		return nil, errors.New("baidupan auth: authorization code and state are required")
	}

	u := *cfg.tokenEndpoint
	query := u.Query()
	query.Set("method", "token")
	query.Set("client_id", cfg.appKey)
	query.Set("client_secret", cfg.secretKey)
	query.Set("code", code)
	query.Set("redirect_uri", cfg.redirectURI)
	query.Set("state", state)
	u.RawQuery = query.Encode()
	requestStarted := time.Now()
	cfg.diagnostics.log("token_request_started", "outcome=started method=GET endpoint_host=%q endpoint_path=%q query_keys=%q client_id_length=%d client_secret_present=%t client_secret_length=%d code_length=%d state_present=%t state_length=%d redirect_uri_hash=%s",
		diagnosticSafeText(cfg.tokenEndpoint.Host, 512, cfg.appKey, cfg.secretKey, code, state),
		diagnosticSafeText(cfg.tokenEndpoint.EscapedPath(), 1024, cfg.appKey, cfg.secretKey, code, state),
		"client_id,client_secret,code,method,redirect_uri,state",
		len(cfg.appKey), cfg.secretKey != "", len(cfg.secretKey), len(code), state != "", len(state), diagnosticHash(cfg.redirectURI))

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=request_build duration_ms=%d", diagnosticDurationMS(requestStarted))
		return nil, errors.New("baidupan auth: create token request")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "BaiduPanGoSDK/1.0")
	response, err := cfg.httpClient.Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=context duration_ms=%d", diagnosticDurationMS(requestStarted))
			return nil, fmt.Errorf("baidupan auth: token request: %w", ctxErr)
		}
		cause := err
		message := err.Error()
		var urlError *url.Error
		if errors.As(err, &urlError) && urlError.Err != nil {
			cause = urlError.Err
			message = urlError.Err.Error()
		}
		redactedMessage := redactValues(message, u.String(), cfg.appKey, cfg.secretKey, code, state, cfg.redirectURI)
		if redactedMessage != message {
			cause = nil
		}
		message = redactedMessage
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=transport duration_ms=%d", diagnosticDurationMS(requestStarted))
		return nil, &tokenRequestError{cause: cause, message: safeText(message, maxOAuthErrorText)}
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxTokenResponseSize+1))
	if err != nil {
		logTokenResponse(cfg, response, len(data), requestStarted, code, state)
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=response_read http_status=%d duration_ms=%d", response.StatusCode, diagnosticDurationMS(requestStarted))
		return nil, fmt.Errorf("baidupan auth: read token response: %w", err)
	}
	if len(data) > maxTokenResponseSize {
		logTokenResponse(cfg, response, len(data), requestStarted, code, state)
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=response_too_large http_status=%d body_bytes=%d duration_ms=%d", response.StatusCode, len(data), diagnosticDurationMS(requestStarted))
		return nil, errors.New("baidupan auth: token response exceeds 1 MiB")
	}
	var envelope tokenEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		logTokenResponse(cfg, response, len(data), requestStarted, code, state)
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=invalid_json http_status=%d body_bytes=%d duration_ms=%d", response.StatusCode, len(data), diagnosticDurationMS(requestStarted))
		return nil, &TokenError{StatusCode: response.StatusCode, Message: "invalid JSON response"}
	}

	payload, err := envelope.payload()
	if err != nil {
		logTokenResponse(cfg, response, len(data), requestStarted, code, state)
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=invalid_token_data http_status=%d body_bytes=%d duration_ms=%d", response.StatusCode, len(data), diagnosticDurationMS(requestStarted))
		return nil, &TokenError{StatusCode: response.StatusCode, Message: "invalid token data response"}
	}
	responseSecrets := []string{cfg.appKey, code, state}
	responseSecrets = append(responseSecrets, tokenPayloadSecrets(envelope.flatPayload())...)
	responseSecrets = append(responseSecrets, tokenPayloadSecrets(payload)...)
	logTokenResponse(cfg, response, len(data), requestStarted, responseSecrets...)
	requestID := redactValues(rawJSONText(envelope.RequestID), append([]string{cfg.secretKey}, responseSecrets...)...)
	newno := redactValues(rawJSONText(envelope.Newno), append([]string{cfg.secretKey}, responseSecrets...)...)
	if envelope.Errno != 0 || envelope.OAuthError != "" || response.StatusCode < 200 || response.StatusCode >= 300 {
		message := firstNonEmpty(envelope.ShowMsg, envelope.Errmsg, envelope.ErrorMsg, envelope.ErrorDescription, envelope.OAuthError)
		if message == "" {
			message = http.StatusText(response.StatusCode)
		}
		message = redactValues(safeText(message, maxOAuthErrorText), append([]string{cfg.secretKey}, responseSecrets...)...)
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=service_response http_status=%d errno=%d newno=%q request_id=%q server_time=%d message=%q duration_ms=%d",
			response.StatusCode, envelope.Errno, safeText(newno, 256), safeText(requestID, 256), envelope.ServerTime, message, diagnosticDurationMS(requestStarted))
		return nil, &TokenError{
			StatusCode: response.StatusCode,
			Errno:      envelope.Errno,
			Message:    message,
			RequestID:  safeText(requestID, 256),
		}
	}
	if strings.TrimSpace(payload.AccessToken) == "" {
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=missing_access_token http_status=%d request_id=%q server_time=%d duration_ms=%d",
			response.StatusCode, safeText(requestID, 256), envelope.ServerTime, diagnosticDurationMS(requestStarted))
		return nil, &TokenError{StatusCode: response.StatusCode, Message: "response contains no access token", RequestID: requestID}
	}
	if payload.ExpiresIn <= 0 || payload.ExpiresIn > maxCredentialLifetimeSeconds {
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=invalid_expiry http_status=%d request_id=%q server_time=%d duration_ms=%d",
			response.StatusCode, safeText(requestID, 256), envelope.ServerTime, diagnosticDurationMS(requestStarted))
		return nil, &TokenError{StatusCode: response.StatusCode, Message: "response contains an invalid expires_in", RequestID: requestID}
	}
	if payload.RedirectURI != "" && !validTokenRedirectURI(payload.RedirectURI, cfg.redirectURI, state) {
		cfg.diagnostics.log("token_exchange_failed", "outcome=failed error_class=invalid_redirect_uri http_status=%d request_id=%q server_time=%d duration_ms=%d",
			response.StatusCode, safeText(requestID, 256), envelope.ServerTime, diagnosticDurationMS(requestStarted))
		return nil, &TokenError{StatusCode: response.StatusCode, Message: "response contains an invalid redirect_uri", RequestID: requestID}
	}
	now := time.Now().UTC()
	credential := &Credential{
		AccessToken:     payload.AccessToken,
		RefreshToken:    payload.RefreshToken,
		ExpiresIn:       payload.ExpiresIn,
		Scope:           payload.Scope,
		RequestedScope:  cfg.scope,
		AuthorizedPaths: append([]string(nil), payload.AuthorizedPaths...),
		RedirectURI:     payload.RedirectURI,
		TokenType:       payload.TokenType,
		SessionKey:      payload.SessionKey,
		SessionSecret:   payload.SessionSecret,
		RequestID:       requestID,
		ServerTime:      envelope.ServerTime,
		CreatedAt:       now,
	}
	credential.ExpiresAt = now.Add(time.Duration(payload.ExpiresIn) * time.Second)
	credential.Extra = extraTokenFields(data)
	cfg.diagnostics.log("token_exchange_succeeded", "outcome=succeeded http_status=%d request_id=%q server_time=%d expires_in=%d scope=%q duration_ms=%d",
		response.StatusCode, safeText(requestID, 256), envelope.ServerTime, payload.ExpiresIn,
		redactValues(safeText(payload.Scope, 512), append([]string{cfg.secretKey}, responseSecrets...)...), diagnosticDurationMS(requestStarted))
	return credential, nil
}

func logTokenResponse(cfg normalizedConfig, response *http.Response, bodyBytes int, started time.Time, secrets ...string) {
	if response == nil {
		return
	}
	header := func(name string) string {
		return safeText(redactValues(response.Header.Get(name), append([]string{cfg.appKey, cfg.secretKey}, secrets...)...), 256)
	}
	cfg.diagnostics.log("token_response_received", "outcome=received http_status=%d content_type=%q content_length=%d body_bytes=%d date=%q logid=%q x_request_id=%q x_bce_request_id=%q yld=%q retry_after=%q duration_ms=%d",
		response.StatusCode, header("Content-Type"), response.ContentLength, bodyBytes, header("Date"), header("Logid"),
		header("X-Request-Id"), header("X-Bce-Request-Id"), header("Yld"), header("Retry-After"), diagnosticDurationMS(started))
}

func tokenPayloadSecrets(payload tokenData) []string {
	secrets := []string{
		payload.AccessToken,
		payload.RefreshToken,
		payload.SessionKey,
		payload.SessionSecret,
		payload.RedirectURI,
	}
	for _, path := range payload.AuthorizedPaths {
		if len(strings.TrimSpace(path)) > 1 {
			secrets = append(secrets, path)
		}
	}
	return secrets
}

func validTokenRedirectURI(raw, configured, state string) bool {
	returned, err := url.Parse(raw)
	if err != nil || returned.User != nil || returned.Fragment != "" {
		return false
	}
	expected, err := url.Parse(configured)
	if err != nil {
		return false
	}
	if returned.Scheme != expected.Scheme || returned.Host != expected.Host || returned.EscapedPath() != expected.EscapedPath() {
		return false
	}
	states, ok := returned.Query()["state"]
	return ok && len(states) == 1 && states[0] == state
}

func extraTokenFields(data []byte) map[string]json.RawMessage {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil
	}
	known := []string{
		"access_token", "refresh_token", "expires_in", "scope", "token_type",
		"session_key", "session_secret", "errno", "show_msg", "errmsg",
		"error_msg", "error", "error_description", "request_id", "server_time", "newno",
		"authorized_paths", "redirect_uri", "data", "client_secret", "code", "state",
	}
	knownSet := make(map[string]struct{}, len(known))
	for _, key := range known {
		knownSet[key] = struct{}{}
	}
	for key := range fields {
		if _, isKnown := knownSet[strings.ToLower(key)]; isKnown {
			delete(fields, key)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func rawJSONText(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return ""
	}
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	return strings.TrimSpace(string(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func redactValues(value string, secrets ...string) string {
	for _, secret := range secrets {
		if secret = strings.TrimSpace(secret); secret != "" {
			value = strings.ReplaceAll(value, secret, "***")
			value = strings.ReplaceAll(value, url.QueryEscape(secret), "***")
			value = strings.ReplaceAll(value, url.PathEscape(secret), "***")
		}
	}
	return value
}
