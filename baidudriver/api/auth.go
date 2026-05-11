package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

const (
	// oauthBaseURL 百度 OAuth 2.0 接口基地址，与网盘 API (pan.baidu.com) 不同。
	oauthBaseURL = "https://openapi.baidu.com"
)

// AuthService 处理百度 OAuth 2.0 授权相关 API。
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
type AuthService service

// doOAuthGet 发送 OAuth GET 请求到 openapi.baidu.com。
// 百度 OAuth API 使用 GET + query string，base URL 与网盘 API 不同。
// 当通过 WithBaseURL 自定义了 base URL 时（通常用于测试），OAuth 请求也使用该地址。
func (c *Client) doOAuthGet(ctx context.Context, path string, params url.Values, v any) (*http.Response, error) {
	base := oauthBaseURL
	if c.rawBaseURL != "" {
		base = c.rawBaseURL
	}
	u, err := url.Parse(base + path)
	if err != nil {
		return nil, fmt.Errorf("parse OAuth path %q: %w", path, err)
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
