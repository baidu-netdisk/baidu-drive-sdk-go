package api

import (
	"context"
	"fmt"
	"net/url"
)

// Code2TokenResponse 授权码换取 token 响应。
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
type Code2TokenResponse struct {
	AccessToken   string `json:"access_token"`
	ExpiresIn     int    `json:"expires_in"`
	RefreshToken  string `json:"refresh_token"`
	Scope         string `json:"scope"`
	SessionKey    string `json:"session_key"`
	SessionSecret string `json:"session_secret"`
}

// Code2Token 使用授权码换取 access_token。
//
// appKey: 应用的 AppKey (即 client_id)
// secretKey: 应用的 SecretKey (即 client_secret)
// code: 用户授权后获得的授权码
// redirectURI: 授权回调地址，需与申请时一致（设备码模式填 "oob"）
//
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
func (s *AuthService) Code2Token(ctx context.Context, appKey, secretKey, code, redirectURI string) (*Code2TokenResponse, error) {
	if appKey == "" || secretKey == "" || code == "" {
		return nil, fmt.Errorf("baidupan: Code2Token params must not be empty")
	}
	if redirectURI == "" {
		redirectURI = "oob"
	}

	q := url.Values{}
	q.Set("grant_type", "authorization_code")
	q.Set("code", code)
	q.Set("client_id", appKey)
	q.Set("client_secret", secretKey)
	q.Set("redirect_uri", redirectURI)

	var resp Code2TokenResponse
	_, err := s.client.doOAuthGet(ctx, "/oauth/2.0/token", q, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
