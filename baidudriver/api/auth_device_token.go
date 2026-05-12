package api

import (
	"context"
	"fmt"
	"net/url"
)

// DeviceTokenResponse 设备令牌响应。
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
type DeviceTokenResponse struct {
	AccessToken   string `json:"access_token"`
	ExpiresIn     int    `json:"expires_in"`
	RefreshToken  string `json:"refresh_token"`
	Scope         string `json:"scope"`
	SessionKey    string `json:"session_key"`
	SessionSecret string `json:"session_secret"`
}

// DeviceToken 使用设备码换取 access_token。
// 在用户完成授权前会返回 "authorization_pending" 错误，调用方需轮询。
//
// appKey: 应用的 AppKey (即 client_id)
// secretKey: 应用的 SecretKey (即 client_secret)
// deviceCode: DeviceCode 接口返回的 device_code
//
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
func (s *AuthService) DeviceToken(ctx context.Context, appKey, secretKey, deviceCode string) (*DeviceTokenResponse, error) {
	if appKey == "" || secretKey == "" || deviceCode == "" {
		return nil, fmt.Errorf("baidupan: DeviceToken params must not be empty")
	}

	q := url.Values{}
	q.Set("grant_type", "device_token")
	q.Set("code", deviceCode)
	q.Set("client_id", appKey)
	q.Set("client_secret", secretKey)

	var resp DeviceTokenResponse
	_, err := s.client.doOAuthGet(ctx, "/oauth/2.0/token", q, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
