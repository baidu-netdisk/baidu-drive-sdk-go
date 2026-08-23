package api

import (
	"context"
	"fmt"
	"net/url"
)

// RefreshTokenResponse 刷新 access token 响应。
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
type RefreshTokenResponse struct {
	AccessToken   string `json:"access_token"`
	ExpiresIn     int    `json:"expires_in"`
	RefreshToken  string `json:"refresh_token"`
	Scope         string `json:"scope"`
	SessionKey    string `json:"session_key"`
	SessionSecret string `json:"session_secret"`
}

// RefreshToken 使用 refresh token 获取新的 access token。
//
// 返回结果包含新的 refresh token，调用方应保存并替换旧值。
//
//   - appKey: 应用的 AppKey (即 client_id)
//   - secretKey: 应用的 SecretKey (即 client_secret)
//   - refreshToken: 换取 Access Token 时候返回的 refresh_token 值，使用一次后失效
//
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
func (s *AuthService) RefreshToken(ctx context.Context, appKey, secretKey, refreshToken string) (*RefreshTokenResponse, error) {
	if appKey == "" || secretKey == "" || refreshToken == "" {
		return nil, fmt.Errorf("baidupan: RefreshToken params must not be empty")
	}

	q := url.Values{}
	q.Set("grant_type", "refresh_token")
	q.Set("refresh_token", refreshToken)
	q.Set("client_id", appKey)
	q.Set("client_secret", secretKey)

	var resp RefreshTokenResponse
	_, err := s.client.doOAuthGet(ctx, "/oauth/2.0/token", q, &resp)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}
