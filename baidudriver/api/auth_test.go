package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestAuthService_DeviceCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		q := r.URL.Query()
		if q.Get("response_type") != "device_code" {
			t.Errorf("response_type = %q, want device_code", q.Get("response_type"))
		}
		if q.Get("client_id") != "test_app_key" {
			t.Errorf("client_id = %q, want test_app_key", q.Get("client_id"))
		}
		if q.Get("scope") != "basic,netdisk" {
			t.Errorf("scope = %q, want basic,netdisk", q.Get("scope"))
		}
		w.Write([]byte(`{
			"device_code": "abc123",
			"user_code": "USER123",
			"verification_url": "https://openapi.baidu.com/device",
			"qrcode_url": "https://openapi.baidu.com/device/qrcode/abc123",
			"expires_in": 300,
			"interval": 5
		}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	resp, err := c.Auth.DeviceCode(context.Background(), "test_app_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeviceCode != "abc123" {
		t.Errorf("DeviceCode = %q, want abc123", resp.DeviceCode)
	}
	if resp.UserCode != "USER123" {
		t.Errorf("UserCode = %q, want USER123", resp.UserCode)
	}
	if resp.QrcodeURL != "https://openapi.baidu.com/device/qrcode/abc123" {
		t.Errorf("QrcodeURL = %q", resp.QrcodeURL)
	}
	if resp.ExpiresIn != 300 {
		t.Errorf("ExpiresIn = %d, want 300", resp.ExpiresIn)
	}
	if resp.Interval != 5 {
		t.Errorf("Interval = %d, want 5", resp.Interval)
	}
}

func TestAuthService_DeviceCode_EmptyAppKey(t *testing.T) {
	c := NewClient()
	_, err := c.Auth.DeviceCode(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty appKey")
	}
}

func TestAuthService_DeviceToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		q := r.URL.Query()
		if q.Get("grant_type") != "device_token" {
			t.Errorf("grant_type = %q, want device_token", q.Get("grant_type"))
		}
		if q.Get("code") != "device_abc" {
			t.Errorf("code = %q, want device_abc", q.Get("code"))
		}
		if q.Get("client_id") != "test_app_key" {
			t.Errorf("client_id = %q, want test_app_key", q.Get("client_id"))
		}
		if q.Get("client_secret") != "test_secret" {
			t.Errorf("client_secret = %q, want test_secret", q.Get("client_secret"))
		}
		w.Write([]byte(`{
			"access_token": "12.token_xxx",
			"expires_in": 2592000,
			"refresh_token": "13.refresh_xxx",
			"scope": "basic netdisk",
			"session_key": "sess_key",
			"session_secret": "sess_secret"
		}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	resp, err := c.Auth.DeviceToken(context.Background(), "test_app_key", "test_secret", "device_abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken != "12.token_xxx" {
		t.Errorf("AccessToken = %q, want 12.token_xxx", resp.AccessToken)
	}
	if resp.ExpiresIn != 2592000 {
		t.Errorf("ExpiresIn = %d, want 2592000", resp.ExpiresIn)
	}
	if resp.RefreshToken != "13.refresh_xxx" {
		t.Errorf("RefreshToken = %q, want 13.refresh_xxx", resp.RefreshToken)
	}
}

func TestAuthService_DeviceToken_EmptyParams(t *testing.T) {
	c := NewClient()
	tests := []struct {
		name       string
		appKey     string
		secretKey  string
		deviceCode string
	}{
		{"empty appKey", "", "secret", "code"},
		{"empty secretKey", "key", "", "code"},
		{"empty deviceCode", "key", "secret", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Auth.DeviceToken(context.Background(), tt.appKey, tt.secretKey, tt.deviceCode)
			if err == nil {
				t.Fatal("expected error for empty param")
			}
		})
	}
}

func TestAuthService_DeviceToken_AuthorizationPending(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":"authorization_pending","error_description":"waiting for user"}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	_, err := c.Auth.DeviceToken(context.Background(), "key", "secret", "code")
	if err == nil {
		t.Fatal("expected error for authorization_pending")
	}
	// 应该是 APIError，errno=-1
	if !IsErrno(err, -1) {
		t.Errorf("expected errno=-1, got: %v", err)
	}
}

func TestAuthService_Code2Token(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		q := r.URL.Query()
		if q.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q, want authorization_code", q.Get("grant_type"))
		}
		if q.Get("code") != "auth_code_123" {
			t.Errorf("code = %q, want auth_code_123", q.Get("code"))
		}
		if q.Get("redirect_uri") != "oob" {
			t.Errorf("redirect_uri = %q, want oob", q.Get("redirect_uri"))
		}
		w.Write([]byte(`{
			"access_token": "12.code_token",
			"expires_in": 2592000,
			"refresh_token": "13.code_refresh",
			"scope": "basic netdisk"
		}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	resp, err := c.Auth.Code2Token(context.Background(), "key", "secret", "auth_code_123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AccessToken != "12.code_token" {
		t.Errorf("AccessToken = %q, want 12.code_token", resp.AccessToken)
	}
	if resp.RefreshToken != "13.code_refresh" {
		t.Errorf("RefreshToken = %q, want 13.code_refresh", resp.RefreshToken)
	}
}

func TestAuthService_Code2Token_WithRedirectURI(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("redirect_uri") != "http://localhost:8787/callback" {
			t.Errorf("redirect_uri = %q, want http://localhost:8787/callback", q.Get("redirect_uri"))
		}
		w.Write([]byte(`{"access_token":"tk","expires_in":100,"refresh_token":"rt"}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	_, err := c.Auth.Code2Token(context.Background(), "key", "secret", "code", "http://localhost:8787/callback")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAuthService_Code2Token_EmptyParams(t *testing.T) {
	c := NewClient()
	tests := []struct {
		name      string
		appKey    string
		secretKey string
		code      string
	}{
		{"empty appKey", "", "secret", "code"},
		{"empty secretKey", "key", "", "code"},
		{"empty code", "key", "secret", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := c.Auth.Code2Token(context.Background(), tt.appKey, tt.secretKey, tt.code, "oob")
			if err == nil {
				t.Fatal("expected error for empty param")
			}
		})
	}
}

func TestAuthService_DeviceCode_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":"invalid_client","error_description":"unknown client id"}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	_, err := c.Auth.DeviceCode(context.Background(), "bad_key")
	if err == nil {
		t.Fatal("expected error for invalid client")
	}
	if !IsErrno(err, -1) {
		t.Errorf("expected errno=-1, got: %v", err)
	}
}

// newOAuthTestClient 创建一个将 OAuth 请求指向 test server 的 Client。
// 因为 OAuth 请求走 oauthBaseURL 而非 client.baseURL，需要用 hook 重定向。
func newOAuthTestClient(testURL string) *Client {
	c := NewClient(WithHTTPClient(&http.Client{
		Transport: &oauthTestTransport{testURL: testURL},
	}))
	return c
}

// oauthTestTransport 将所有 openapi.baidu.com 请求重定向到 test server。
type oauthTestTransport struct {
	testURL string
}

func (t *oauthTestTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 将 openapi.baidu.com 替换为 test server URL
	testU, _ := url.Parse(t.testURL)
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = testU.Scheme
	req2.URL.Host = testU.Host
	return http.DefaultTransport.RoundTrip(req2)
}

func TestAuthService_Code2Token_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error":"invalid_grant","error_description":"code expired"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, err := c.Auth.Code2Token(context.Background(), "key", "secret", "expired_code", "oob")
	if err == nil {
		t.Fatal("expected error for expired code")
	}
	if !IsErrno(err, -1) {
		t.Errorf("expected errno=-1, got: %v", err)
	}
}

func TestClient_doOAuthGet_WithBaseURL(t *testing.T) {
	// 测试 rawBaseURL 非空时 doOAuthGet 使用自定义 base URL
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Write([]byte(`{
			"device_code": "dc_from_custom_url",
			"user_code": "UC123",
			"verification_url": "https://example.com",
			"qrcode_url": "",
			"expires_in": 100,
			"interval": 3
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Auth.DeviceCode(context.Background(), "test_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DeviceCode != "dc_from_custom_url" {
		t.Errorf("DeviceCode = %q, want dc_from_custom_url", resp.DeviceCode)
	}
}

func TestClient_doOAuthGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if q := r.URL.Query().Get("test"); q != "value" {
			t.Errorf("test = %q, want value", q)
		}
		w.Write([]byte(`{"errno":0,"name":"ok"}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	var result struct {
		Name string `json:"name"`
	}
	q := url.Values{}
	q.Set("test", "value")
	_, err := c.doOAuthGet(context.Background(), "/test", q, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "ok" {
		t.Errorf("name = %q, want ok", result.Name)
	}
}

func TestClient_doOAuthGet_NilParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	c := newOAuthTestClient(ts.URL)
	var result map[string]any
	_, err := c.doOAuthGet(context.Background(), "/test", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_doOAuthGet_URLParseError(t *testing.T) {
	// Trigger url.Parse error by constructing a base+path that is invalid.
	// We need rawBaseURL to be set to something that when concatenated with path
	// produces an invalid URL. A NUL byte in the URL triggers a parse error.
	c := NewClient(WithBaseURL("https://example.com"))
	// Override rawBaseURL to something that will cause url.Parse to fail
	c.rawBaseURL = "://\x00"
	var result map[string]any
	_, err := c.doOAuthGet(context.Background(), "/test", nil, &result)
	if err == nil {
		t.Fatal("expected error for invalid OAuth URL")
	}
}
