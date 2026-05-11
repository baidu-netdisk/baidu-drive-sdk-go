package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	defaultBaseURL    = "https://pan.baidu.com"
	defaultPCSBaseURL = "https://d.pcs.baidu.com"
	defaultUserAgent  = "BaiduPanGoSDK/1.0"
)

// service is the base struct embedded by all service implementations.
type service struct {
	client *Client
}

// Client manages communication with the Baidu Netdisk API.
type Client struct {
	httpClient  *http.Client
	baseURL     *url.URL
	pcsBaseURL  *url.URL // PCS upload endpoint (d.pcs.baidu.com)
	userAgent   string
	debug       bool
	accessToken string
	apiKey      string
	rawBaseURL  string
	rawPCSURL   string
	logger      *log.Logger // debug logger, nil means no logging

	common service // reuse a single struct instead of allocating one for each service

	File        *FileService
	FileManager *FileManagerService
	Auth        *AuthService
	Nas         *NasService
	Upload      *UploadService
	Download    *DownloadService
}

// NewClient creates a new Baidu Netdisk API client.
// It panics if the base URL is malformed.
func NewClient(opts ...Option) *Client {
	c := &Client{
		userAgent: defaultUserAgent,
	}

	for _, opt := range opts {
		opt(c)
	}

	// Parse base URL
	baseURLStr := defaultBaseURL
	if c.rawBaseURL != "" {
		baseURLStr = c.rawBaseURL
	}
	var err error
	c.baseURL, err = url.Parse(baseURLStr)
	if err != nil {
		panic("baidupan: invalid base URL: " + err.Error())
	}

	// Parse PCS base URL
	pcsURLStr := defaultPCSBaseURL
	if c.rawPCSURL != "" {
		pcsURLStr = c.rawPCSURL
	}
	c.pcsBaseURL, err = url.Parse(pcsURLStr)
	if err != nil {
		panic("baidupan: invalid PCS base URL: " + err.Error())
	}

	// Set up HTTP client with token transport
	if c.httpClient == nil {
		c.httpClient = &http.Client{}
	}
	if c.accessToken != "" {
		transport := c.httpClient.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		c.httpClient.Transport = &tokenTransport{
			token: c.accessToken,
			base:  transport,
		}
	}
	if c.apiKey != "" {
		transport := c.httpClient.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		c.httpClient.Transport = &apiKeyTransport{
			apiKey: c.apiKey,
			base:   transport,
		}
	}

	c.common.client = c
	c.File = (*FileService)(&c.common)
	c.FileManager = (*FileManagerService)(&c.common)
	c.Auth = (*AuthService)(&c.common)
	c.Nas = (*NasService)(&c.common)
	c.Upload = (*UploadService)(&c.common)
	c.Download = (*DownloadService)(&c.common)

	return c
}

// logf writes a debug message if debug mode is enabled.
func (c *Client) logf(format string, args ...any) {
	if !c.debug {
		return
	}
	if c.logger != nil {
		c.logger.Printf(format, args...)
	} else {
		log.Printf("[baidupan] "+format, args...)
	}
}

const maxResponseSize = 10 * 1024 * 1024 // 10MB

// Do sends an API request and decodes the JSON response into v.
// It checks for API errors (non-zero errno) and returns them as *APIError.
// The response body is always consumed and closed; callers should not read resp.Body.
func (c *Client) Do(ctx context.Context, req *http.Request, v any) (*http.Response, error) {
	req = req.WithContext(ctx)

	if c.debug {
		// Redact access_token from debug output
		debugURL := *req.URL
		q := debugURL.Query()
		if q.Get("access_token") != "" {
			q.Set("access_token", "***")
			debugURL.RawQuery = q.Encode()
		}
		c.logf("%s %s", req.Method, debugURL.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if v == nil {
		return resp, nil
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return resp, fmt.Errorf("read response body: %w", err)
	}

	if c.debug {
		c.logf("Response [%d]: %s", resp.StatusCode, string(data))
	}

	// 构建诊断信息：脱敏 URL + 响应体摘要
	redactedURL := redactURL(req.URL)
	respBodySummary := truncateBody(string(data), 1024)

	// Check for API error
	// 百度 API 有四种错误格式:
	//   1. {"errno": -6, "errmsg": "..."}           — 网盘 API
	//   2. {"error_code": 111, "error_msg": "..."}   — 旧版 OAuth
	//   3. {"error": "invalid_client", "error_description": "..."}  — OAuth 2.0
	//   4. {"error_no": -6, "error_msg": "..."}     — /xpan/unisearch 等接口
	var errResp struct {
		Errno            int    `json:"errno"`
		Errmsg           string `json:"errmsg"`
		ErrorCode        int    `json:"error_code"`
		ErrorMsg         string `json:"error_msg"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
		ErrorNo          int    `json:"error_no"`
	}
	if err := json.Unmarshal(data, &errResp); err == nil {
		// OAuth 2.0 错误格式: {"error":"invalid_client","error_description":"..."}
		if errResp.Error != "" {
			return resp, &APIError{
				Errno:        -1,
				Errmsg:       errResp.Error + ": " + errResp.ErrorDescription,
				Response:     resp,
				Method:       req.Method,
				URL:          redactedURL,
				ResponseBody: respBodySummary,
			}
		}
		errno := errResp.Errno
		if errno == 0 {
			errno = errResp.ErrorCode
		}
		// 第四种错误格式: {"error_no": -6, "error_msg": "..."}
		if errno == 0 && errResp.ErrorNo != 0 {
			errno = errResp.ErrorNo
		}
		if errno != 0 {
			msg := errResp.Errmsg
			if msg == "" {
				msg = errResp.ErrorMsg
			}
			return resp, &APIError{
				Errno:        errno,
				Errmsg:       msg,
				Response:     resp,
				Method:       req.Method,
				URL:          redactedURL,
				ResponseBody: respBodySummary,
			}
		}
	}

	// HTTP 非 2xx 兜底：如果 JSON 错误字段全为零值但 HTTP 状态码异常，
	// 将响应体内容包含在错误中，方便业务排查问题。
	if resp.StatusCode >= 400 {
		body := truncateBody(string(data), 512)
		return resp, &APIError{
			Errno:        ErrnoHTTPError,
			Errmsg:       fmt.Sprintf("HTTP %d: %s", resp.StatusCode, body),
			Response:     resp,
			Method:       req.Method,
			URL:          redactedURL,
			ResponseBody: respBodySummary,
		}
	}

	if err := json.Unmarshal(data, v); err != nil {
		return resp, fmt.Errorf("decode response: %w", err)
	}

	return resp, nil
}

// doGet is a convenience for GET requests with query parameters.
func (c *Client) doGet(ctx context.Context, path string, params url.Values, v any) (*http.Response, error) {
	u, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)

	return c.Do(ctx, req, v)
}

// doPost 是 POST 请求的便捷方法，使用 application/x-www-form-urlencoded 编码。
// params 用于 URL query string，body 用于请求体。
func (c *Client) doPost(ctx context.Context, path string, params url.Values, body url.Values, v any) (*http.Response, error) {
	return c.doPostTo(ctx, c.baseURL, path, params, body, v)
}

// doPostTo 是 doPost 的通用版本，允许指定 base URL。
func (c *Client) doPostTo(ctx context.Context, base *url.URL, path string, params url.Values, body url.Values, v any) (*http.Response, error) {
	u, err := base.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	var bodyStr string
	if body != nil {
		bodyStr = body.Encode()
	}
	if c.debug {
		c.logf("Request body: %s", bodyStr)
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", c.userAgent)

	return c.Do(ctx, req, v)
}

// doPostJSON 是 POST 请求的便捷方法，使用 application/json 编码。
// params 用于 URL query string，body 会被序列化为 JSON 请求体。
func (c *Client) doPostJSON(ctx context.Context, path string, params url.Values, body any, v any) (*http.Response, error) {
	u, err := c.baseURL.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	if c.debug {
		c.logf("Request body: %s", string(jsonData))
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	return c.Do(ctx, req, v)
}

// redactURL 返回脱敏后的 URL 字符串（access_token → "***"）。
func redactURL(u *url.URL) string {
	q := u.Query()
	if q.Get("access_token") != "" {
		q.Set("access_token", "***")
	}
	redacted := *u
	redacted.RawQuery = q.Encode()
	return redacted.String()
}

// truncateBody 截断响应体至 maxLen 字节，按 UTF-8 rune 边界对齐。
func truncateBody(body string, maxLen int) string {
	body = strings.TrimSpace(body)
	if len(body) <= maxLen {
		return body
	}
	truncated := body[:maxLen]
	for !utf8.ValidString(truncated) && len(truncated) > 0 {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated + "...(truncated)"
}

// doGetStream 发送 GET 请求并返回原始响应流（不消费 body）。
// 调用方负责关闭返回的 io.ReadCloser。
// 适用于大文件下载场景，避免将响应体全部读入内存。
// fullURL 为完整 URL 字符串（如 dlink），不经过 baseURL 拼接。
func (c *Client) doGetStream(ctx context.Context, fullURL string, userAgent string) (body io.ReadCloser, contentLength int64, err error) {
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req = req.WithContext(ctx)

	ua := c.userAgent
	if userAgent != "" {
		ua = userAgent
	}
	req.Header.Set("User-Agent", ua)

	if c.debug {
		debugURL := *req.URL
		q := debugURL.Query()
		if q.Get("access_token") != "" {
			q.Set("access_token", "***")
			debugURL.RawQuery = q.Encode()
		}
		c.logf("GET stream %s", debugURL.String())
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, 0, &APIError{
			Errno:        ErrnoHTTPError,
			Errmsg:       fmt.Sprintf("HTTP %d: %s", resp.StatusCode, truncateBody(string(data), 512)),
			Response:     resp,
			Method:       req.Method,
			URL:          redactURL(req.URL),
			ResponseBody: truncateBody(string(data), 1024),
		}
	}

	return resp.Body, resp.ContentLength, nil
}

// doPostMultipart 发送 multipart/form-data POST 请求。
// base 指定 base URL（如 pcsBaseURL 用于上传），path 是接口路径，
// params 是 URL query string，fieldName 是 multipart field 名称，
// fileName 是上传文件名，reader 是文件内容，v 是响应解析目标。
//
// 注意: 百度 PCS 不支持 chunked transfer-encoding，因此需要先将 multipart
// 数据写入 buffer 以获取 Content-Length。每个分片最大 4MB，内存开销可接受。
func (c *Client) doPostMultipart(ctx context.Context, base *url.URL, path string, params url.Values, fieldName, fileName string, reader io.Reader, v any) (*http.Response, error) {
	u, err := base.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("parse path %q: %w", path, err)
	}
	if params != nil {
		u.RawQuery = params.Encode()
	}

	// 将 multipart 数据写入 buffer 以获取精确的 Content-Length
	// （百度 PCS 不支持 chunked transfer-encoding，errno=31211）
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, reader); err != nil {
		return nil, fmt.Errorf("copy file data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, u.String(), &buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = int64(buf.Len())
	req.Header.Set("User-Agent", c.userAgent)

	if c.debug {
		c.logf("Multipart upload: %s %s field=%s file=%s size=%d", req.Method, u.String(), fieldName, fileName, req.ContentLength)
	}

	return c.Do(ctx, req, v)
}
