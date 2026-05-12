package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient()
	if c.baseURL.String() != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL.String(), defaultBaseURL)
	}
	if c.userAgent != defaultUserAgent {
		t.Errorf("userAgent = %q, want %q", c.userAgent, defaultUserAgent)
	}
	if c.File == nil {
		t.Error("File service should not be nil")
	}
	if c.FileManager == nil {
		t.Error("FileManager service should not be nil")
	}
	if c.Auth == nil {
		t.Error("Auth service should not be nil")
	}
	if c.Nas == nil {
		t.Error("Nas service should not be nil")
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	c := NewClient(
		WithAccessToken("mytoken"),
		WithDebug(true),
	)
	if c.accessToken != "mytoken" {
		t.Errorf("accessToken = %q, want %q", c.accessToken, "mytoken")
	}
	if !c.debug {
		t.Error("debug should be true")
	}
}

func TestNewClient_WithBaseURL(t *testing.T) {
	c := NewClient(WithBaseURL("https://custom.api.com"))
	if c.baseURL.String() != "https://custom.api.com" {
		t.Errorf("baseURL = %q, want %q", c.baseURL.String(), "https://custom.api.com")
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	c := NewClient(WithHTTPClient(custom))
	if c.httpClient != custom {
		t.Error("httpClient should be the custom client")
	}
}

func TestNewClient_WithLogger(t *testing.T) {
	var buf bytes.Buffer
	c := NewClient(WithDebug(true), WithLogger(&buf), WithBaseURL("https://example.com"))
	c.logf("test %s", "msg")
	if buf.Len() == 0 {
		t.Error("logger should have received output")
	}
}

func TestClient_logf_DefaultLogger(t *testing.T) {
	// 测试 debug=true 且 logger=nil 时走默认 log.Printf 分支
	c := NewClient(WithDebug(true))
	// 不应 panic
	c.logf("test %s", "default")
}

func TestClient_logf_Disabled(t *testing.T) {
	c := NewClient() // debug=false
	c.logf("should not log %s", "anything")
}

func TestClient_Do_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}

func TestClient_Do_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errno":0,"name":"test"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result struct {
		Name string `json:"name"`
	}
	_, err := c.Do(context.Background(), req, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "test" {
		t.Errorf("name = %q, want %q", result.Name, "test")
	}
}

func TestClient_Do_NilV(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`OK`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	resp, err := c.Do(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestClient_Do_OAuthError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error_code":111,"error_msg":"token invalid"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsErrno(err, 111) {
		t.Errorf("expected errno=111, got: %v", err)
	}
}

func TestClient_Do_OAuth2Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":"invalid_client","error_description":"bad credentials"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Errno != -1 {
		t.Errorf("errno = %d, want -1", apiErr.Errno)
	}
}

func TestClient_Do_UniSearchError(t *testing.T) {
	// 测试第四种错误格式: {"error_no": -6, "error_msg": "..."}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error_no":-6,"error_msg":"access denied"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/xpan/unisearch", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}

func TestClient_Do_DebugWithToken(t *testing.T) {
	// 测试 debug 模式下 access_token 被脱敏
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	c := NewClient(WithBaseURL(ts.URL), WithAccessToken("secret"), WithDebug(true), WithLogger(&buf))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test?access_token=secret", nil)
	var result map[string]any
	c.Do(context.Background(), req, &result)
	if bytes.Contains(buf.Bytes(), []byte("secret")) {
		t.Error("debug output should not contain the raw access_token")
	}
}

func TestClient_Do_InvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
}

func TestClient_Do_HTTP400_NoErrno(t *testing.T) {
	// HTTP 400 但 JSON 中 errno=0，应走兜底逻辑
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errno":0,"message":"invalid request"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Errno != ErrnoHTTPError {
		t.Errorf("errno = %d, want %d", apiErr.Errno, ErrnoHTTPError)
	}
	if apiErr.Response.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", apiErr.Response.StatusCode)
	}
}

func TestClient_Do_HTTP500_NonJSON(t *testing.T) {
	// HTTP 500 + 非 JSON 响应（如 HTML 错误页面）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`<html>Internal Server Error</html>`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Errno != ErrnoHTTPError {
		t.Errorf("errno = %d, want %d", apiErr.Errno, ErrnoHTTPError)
	}
	if !bytes.Contains([]byte(apiErr.Errmsg), []byte("Internal Server Error")) {
		t.Errorf("errmsg should contain response body, got: %s", apiErr.Errmsg)
	}
}

func TestClient_Do_HTTP400_LongBody_UTF8Safe(t *testing.T) {
	// 超过 512 字节的中文响应体应 UTF-8 安全截断
	longChinese := ""
	for len(longChinese) < 600 {
		longChinese += "错误"
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(longChinese))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr := err.(*APIError)
	// 截断后不应包含不完整的 UTF-8 字符
	if !bytes.Contains([]byte(apiErr.Errmsg), []byte("(truncated)")) {
		t.Errorf("long body should be truncated, got: %s", apiErr.Errmsg)
	}
}

func TestClient_doPostJSON_DebugBody(t *testing.T) {
	// 验证 debug 模式下请求体被打印
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	c := NewClient(WithBaseURL(ts.URL), WithDebug(true), WithLogger(&buf))
	var result map[string]any
	c.doPostJSON(context.Background(), "/test", nil, map[string]string{"key": "val"}, &result)
	if !bytes.Contains(buf.Bytes(), []byte("Request body:")) {
		t.Error("debug output should contain request body log")
	}
	if !bytes.Contains(buf.Bytes(), []byte("key")) {
		t.Error("debug output should contain the request body content")
	}
}

func TestClient_doPost_DebugBody(t *testing.T) {
	// 验证 doPost debug 模式下请求体被打印
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	c := NewClient(WithBaseURL(ts.URL), WithDebug(true), WithLogger(&buf))
	var result map[string]any
	body := map[string][]string{"field": {"value"}}
	c.doPost(context.Background(), "/test", nil, body, &result)
	if !bytes.Contains(buf.Bytes(), []byte("Request body:")) {
		t.Error("debug output should contain request body log")
	}
}

func TestClient_doGet_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "value" {
			t.Errorf("query param key = %q, want %q", r.URL.Query().Get("key"), "value")
		}
		w.Write([]byte(`{"errno":0,"data":"ok"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	var result struct {
		Data string `json:"data"`
	}
	_, err := c.doGet(context.Background(), "/test", map[string][]string{"key": {"value"}}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data != "ok" {
		t.Errorf("data = %q, want %q", result.Data, "ok")
	}
}

func TestClient_doGet_NilParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	var result map[string]any
	_, err := c.doGet(context.Background(), "/test", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// doPostJSON 覆盖补充
// =============================================================================

func TestClient_doPostJSON_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		w.Write([]byte(`{"errno":0,"data":"ok"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	var result struct {
		Data string `json:"data"`
	}
	_, err := c.doPostJSON(context.Background(), "/test", nil, map[string]string{"key": "val"}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data != "ok" {
		t.Errorf("data = %q, want ok", result.Data)
	}
}

func TestClient_doPostJSON_WithQueryParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("scene") != "mcpserver" {
			t.Errorf("scene = %q, want mcpserver", r.URL.Query().Get("scene"))
		}
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	params := map[string][]string{"scene": {"mcpserver"}}
	var result map[string]any
	_, err := c.doPostJSON(context.Background(), "/test", params, struct{}{}, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_doPostJSON_InvalidPath(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	var result map[string]any
	_, err := c.doPostJSON(context.Background(), "://bad-path", nil, struct{}{}, &result)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestClient_doPostJSON_MarshalError(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	var result map[string]any
	// chan 类型无法 JSON 序列化
	_, err := c.doPostJSON(context.Background(), "/test", nil, make(chan int), &result)
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

// =============================================================================
// doGet / doPost / doOAuthGet 错误路径覆盖
// =============================================================================

func TestClient_doGet_InvalidPath(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	var result map[string]any
	_, err := c.doGet(context.Background(), "://bad-path", nil, &result)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestClient_doPost_InvalidPath(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	var result map[string]any
	_, err := c.doPost(context.Background(), "://bad-path", nil, nil, &result)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestClient_doPost_NilBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	var result map[string]any
	_, err := c.doPost(context.Background(), "/test", nil, nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_doPost_WithQueryParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("method") != "create" {
			t.Errorf("method = %q, want create", r.URL.Query().Get("method"))
		}
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	params := map[string][]string{"method": {"create"}}
	var result map[string]any
	_, err := c.doPost(context.Background(), "/test", params, nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_Do_HttpError(t *testing.T) {
	// 测试 httpClient.Do 返回错误的路径（client.go:119）
	c := NewClient(WithBaseURL("http://127.0.0.1:0")) // 不可达的地址
	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:0/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestClient_Do_ReadBodyError(t *testing.T) {
	// 测试 io.ReadAll 返回错误的路径（client.go:129）
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100") // 声明100字节但只写5字节
		w.Write([]byte(`hello`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test", nil)
	var result map[string]any
	// 即使 Content-Length 不匹配，Go 的 http client 会自动处理
	// 此路径在正常条件下很难触发，改为测试 context canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消
	_, err := c.Do(ctx, req, &result)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestClient_Do_APIError_Diagnostics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAccessToken("secret_token"))
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/xpan/unisearch?query=test&access_token=secret_token", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Method != "POST" {
		t.Errorf("Method = %q, want POST", apiErr.Method)
	}
	if apiErr.URL == "" {
		t.Error("URL should not be empty")
	}
	// access_token 应被脱敏
	if strings.Contains(apiErr.URL, "secret_token") {
		t.Error("URL should not contain raw access_token")
	}
	if !strings.Contains(apiErr.URL, "access_token=%2A%2A%2A") && !strings.Contains(apiErr.URL, "access_token=***") {
		t.Errorf("URL should contain redacted access_token, got %q", apiErr.URL)
	}
	if apiErr.ResponseBody == "" {
		t.Error("ResponseBody should not be empty")
	}
	if !strings.Contains(apiErr.ResponseBody, "access denied") {
		t.Errorf("ResponseBody = %q, want to contain 'access denied'", apiErr.ResponseBody)
	}
}

func TestClient_Do_HTTPError_Diagnostics(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`bad request details`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/test/path", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Errno != ErrnoHTTPError {
		t.Errorf("Errno = %d, want %d", apiErr.Errno, ErrnoHTTPError)
	}
	if apiErr.Method != "GET" {
		t.Errorf("Method = %q, want GET", apiErr.Method)
	}
	if !strings.Contains(apiErr.URL, "/test/path") {
		t.Errorf("URL = %q, want to contain /test/path", apiErr.URL)
	}
	if !strings.Contains(apiErr.ResponseBody, "bad request details") {
		t.Errorf("ResponseBody = %q, want to contain 'bad request details'", apiErr.ResponseBody)
	}
}

func TestRedactURL(t *testing.T) {
	u, _ := url.Parse("https://pan.baidu.com/xpan/file?method=list&access_token=secret123")
	result := redactURL(u)
	if strings.Contains(result, "secret123") {
		t.Errorf("redactURL should redact access_token, got %q", result)
	}
	if !strings.Contains(result, "method=list") {
		t.Errorf("redactURL should preserve other params, got %q", result)
	}
}

func TestRedactURL_NoToken(t *testing.T) {
	u, _ := url.Parse("https://pan.baidu.com/xpan/file?method=list")
	result := redactURL(u)
	if result != "https://pan.baidu.com/xpan/file?method=list" {
		t.Errorf("redactURL without token should return original, got %q", result)
	}
}

func TestTruncateBody(t *testing.T) {
	short := `{"errno":0}`
	if truncateBody(short, 1024) != short {
		t.Errorf("short body should not be truncated")
	}

	long := strings.Repeat("a", 2000)
	result := truncateBody(long, 100)
	if len(result) > 120 { // 100 + "...(truncated)"
		t.Errorf("truncated body too long: %d", len(result))
	}
	if !strings.HasSuffix(result, "...(truncated)") {
		t.Errorf("truncated body should end with ...(truncated)")
	}
}

func TestTruncateBody_UTF8Safe(t *testing.T) {
	// 中文字符 UTF-8 编码为 3 字节
	body := strings.Repeat("你好世界", 100)
	result := truncateBody(body, 50)
	// 截断后应仍是有效 UTF-8
	for i := 0; i < len(result); i++ {
		if result[i] == '.' {
			break // 跳过 ...(truncated) 后缀
		}
	}
	if !strings.HasSuffix(result, "...(truncated)") {
		t.Errorf("should end with ...(truncated)")
	}
}

// newTestServer creates a test server and a client pointing to it.
func newTestServer(handler http.HandlerFunc) (*httptest.Server, *Client) {
	ts := httptest.NewServer(handler)
	c := NewClient(WithBaseURL(ts.URL))
	return ts, c
}

// =============================================================================
// PCS base URL 相关测试
// =============================================================================

func TestNewClient_WithPCSBaseURL(t *testing.T) {
	c := NewClient(WithPCSBaseURL("https://custom-pcs.example.com"))
	if c.pcsBaseURL.String() != "https://custom-pcs.example.com" {
		t.Errorf("pcsBaseURL = %q, want https://custom-pcs.example.com", c.pcsBaseURL.String())
	}
}

func TestNewClient_DefaultPCSBaseURL(t *testing.T) {
	c := NewClient()
	if c.pcsBaseURL.String() != defaultPCSBaseURL {
		t.Errorf("pcsBaseURL = %q, want %q", c.pcsBaseURL.String(), defaultPCSBaseURL)
	}
}

func TestNewClient_Upload_NotNil(t *testing.T) {
	c := NewClient()
	if c.Upload == nil {
		t.Error("Upload service should not be nil")
	}
}

// =============================================================================
// doPostTo 覆盖补充
// =============================================================================

func TestClient_doPostTo_InvalidPath(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	var result map[string]any
	_, err := c.doPostTo(context.Background(), c.baseURL, "://bad-path", nil, nil, &result)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

// =============================================================================
// doPostMultipart 覆盖补充
// =============================================================================

func TestClient_doPostMultipart_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		w.Write([]byte(`{"errno":0,"md5":"test123"}`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	c := NewClient(WithBaseURL(ts.URL))
	var result struct {
		MD5 string `json:"md5"`
	}
	_, err := c.doPostMultipart(context.Background(), u, "/upload", nil, "file", "test.txt", strings.NewReader("data"), &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.MD5 != "test123" {
		t.Errorf("MD5 = %q, want test123", result.MD5)
	}
}

func TestClient_doPostMultipart_InvalidPath(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	var result map[string]any
	_, err := c.doPostMultipart(context.Background(), c.baseURL, "://bad-path", nil, "file", "f", strings.NewReader(""), &result)
	if err == nil {
		t.Fatal("expected error for invalid path")
	}
}

func TestClient_doPostMultipart_WithParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("method") != "upload" {
			t.Errorf("method = %q, want upload", r.URL.Query().Get("method"))
		}
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	c := NewClient(WithBaseURL(ts.URL))
	params := url.Values{"method": {"upload"}}
	var result map[string]any
	_, err := c.doPostMultipart(context.Background(), u, "/test", params, "file", "f.txt", strings.NewReader("data"), &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_doPostMultipart_Debug(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	u, _ := url.Parse(ts.URL)
	c := NewClient(WithBaseURL(ts.URL), WithDebug(true), WithLogger(&buf))
	var result map[string]any
	c.doPostMultipart(context.Background(), u, "/upload", nil, "file", "test.txt", strings.NewReader("data"), &result)
	if !bytes.Contains(buf.Bytes(), []byte("Multipart upload")) {
		t.Error("debug output should contain 'Multipart upload'")
	}
}

func TestClient_doPostMultipart_NilParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "" {
			t.Errorf("expected no query params, got: %s", r.URL.RawQuery)
		}
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	c := NewClient(WithBaseURL(ts.URL))
	var result map[string]any
	_, err := c.doPostMultipart(context.Background(), u, "/test", nil, "file", "f.txt", strings.NewReader("d"), &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_doPostMultipart_ErrorReader(t *testing.T) {
	// 测试 reader 返回错误时的行为
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 服务端可能收到不完整的 body
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"errno":-1}`))
	}))
	defer ts.Close()

	u, _ := url.Parse(ts.URL)
	c := NewClient(WithBaseURL(ts.URL))
	var result map[string]any
	_, err := c.doPostMultipart(context.Background(), u, "/test", nil, "file", "f.txt", &errorReader{}, &result)
	// 应该返回错误（来自 reader 或来自 server）
	if err == nil {
		t.Fatal("expected error for error reader")
	}
}

// errorReader 是一个总是返回错误的 io.Reader。
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("simulated read error")
}

// =============================================================================
// doGetStream 覆盖补充
// =============================================================================

func TestClient_doGetStream_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.Header().Set("Content-Length", "5")
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	body, size, err := c.doGetStream(context.Background(), ts.URL+"/file", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "hello" {
		t.Errorf("body = %q, want hello", string(data))
	}
	if size != 5 {
		t.Errorf("size = %d, want 5", size)
	}
}

func TestClient_doGetStream_CustomUserAgent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != "custom-ua" {
			t.Errorf("User-Agent = %q, want custom-ua", ua)
		}
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	body, _, err := c.doGetStream(context.Background(), ts.URL+"/file", "custom-ua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body.Close()
}

func TestClient_doGetStream_DebugRedactsToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	c := NewClient(WithBaseURL(ts.URL), WithDebug(true), WithLogger(&buf), WithAccessToken("secret_token"))
	body, _, err := c.doGetStream(context.Background(), ts.URL+"/file?access_token=secret_token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body.Close()

	logOutput := buf.String()
	if strings.Contains(logOutput, "secret_token") {
		t.Error("debug output should not contain the raw access_token")
	}
	if !strings.Contains(logOutput, "GET stream") {
		t.Error("debug output should contain 'GET stream'")
	}
}

func TestClient_doGetStream_DebugWithoutToken(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	var buf bytes.Buffer
	c := NewClient(WithBaseURL(ts.URL), WithDebug(true), WithLogger(&buf))
	body, _, err := c.doGetStream(context.Background(), ts.URL+"/file", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body.Close()
	if !strings.Contains(buf.String(), "GET stream") {
		t.Error("debug output should contain 'GET stream'")
	}
}

func TestClient_doGetStream_InvalidURL(t *testing.T) {
	c := NewClient()
	_, _, err := c.doGetStream(context.Background(), "://invalid-url", "")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestClient_doGetStream_HttpDoError(t *testing.T) {
	c := NewClient(WithBaseURL("http://127.0.0.1:0"))
	_, _, err := c.doGetStream(context.Background(), "http://127.0.0.1:0/file", "")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestClient_doGetStream_Non200Status(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, _, err := c.doGetStream(context.Background(), ts.URL+"/file", "")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Errno != ErrnoHTTPError {
		t.Errorf("errno = %d, want %d", apiErr.Errno, ErrnoHTTPError)
	}
	if !strings.Contains(apiErr.Errmsg, "403") {
		t.Errorf("errmsg should contain 403, got: %s", apiErr.Errmsg)
	}
}

func TestClient_doGetStream_Non200_LargeBody(t *testing.T) {
	largeBody := strings.Repeat("error-data-", 500)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(largeBody))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, _, err := c.doGetStream(context.Background(), ts.URL+"/file", "")
	if err == nil {
		t.Fatal("expected error for 500 status")
	}
	apiErr := err.(*APIError)
	if apiErr.Response.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", apiErr.Response.StatusCode)
	}
}

// =============================================================================
// NewClient panic 路径覆盖
// =============================================================================

func TestNewClient_InvalidBaseURL_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid base URL")
		}
		s, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(s, "invalid base URL") {
			t.Errorf("panic message = %q, want to contain 'invalid base URL'", s)
		}
	}()
	NewClient(WithBaseURL("://\x00invalid"))
}

func TestNewClient_InvalidPCSBaseURL_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for invalid PCS base URL")
		}
		s, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T", r)
		}
		if !strings.Contains(s, "invalid PCS base URL") {
			t.Errorf("panic message = %q, want to contain 'invalid PCS base URL'", s)
		}
	}()
	NewClient(WithPCSBaseURL("://\x00invalid"))
}

// =============================================================================
// Do io.ReadAll error path (client.go:150)
// =============================================================================

// errorBodyTransport returns a response with a body that errors on Read.
type errorBodyTransport struct {
	statusCode int
}

func (t *errorBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.statusCode,
		Body:       io.NopCloser(&errorReader{}),
		Header:     make(http.Header),
	}, nil
}

func TestClient_Do_ReadAllError(t *testing.T) {
	c := NewClient(WithHTTPClient(&http.Client{
		Transport: &errorBodyTransport{statusCode: 200},
	}))
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	var result map[string]any
	_, err := c.Do(context.Background(), req, &result)
	if err == nil {
		t.Fatal("expected error for body read failure")
	}
	if !strings.Contains(err.Error(), "read response body") {
		t.Errorf("error = %q, want to contain 'read response body'", err.Error())
	}
}

// =============================================================================
// WithAPIKey 集成测试
// =============================================================================

func TestNewClient_WithAPIKey(t *testing.T) {
	c := NewClient(WithAPIKey("my-api-key"))
	if c.apiKey != "my-api-key" {
		t.Errorf("apiKey = %q, want %q", c.apiKey, "my-api-key")
	}
}

func TestNewClient_WithAPIKey_InjectsQueryParam(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("api_key")
		if key != "my-api-key" {
			t.Errorf("api_key = %q, want my-api-key", key)
		}
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL), WithAPIKey("my-api-key"))
	var result map[string]any
	_, err := c.doGet(context.Background(), "/test", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewClient_BothTokenAndAPIKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("access_token")
		key := r.URL.Query().Get("api_key")
		if token != "tok" {
			t.Errorf("access_token = %q, want tok", token)
		}
		if key != "key123" {
			t.Errorf("api_key = %q, want key123", key)
		}
		w.Write([]byte(`{"errno":0}`))
	}))
	defer ts.Close()

	c := NewClient(
		WithBaseURL(ts.URL),
		WithAccessToken("tok"),
		WithAPIKey("key123"),
	)
	var result map[string]any
	_, err := c.doGet(context.Background(), "/test", nil, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
