package api

import (
	"context"
	"fmt"
	"net/url"
)

// DeviceCodeResponse 设备码请求响应。
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	QrcodeURL       string `json:"qrcode_url"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceCode 获取设备码，用于设备码授权流程。
// 用户需访问 VerificationURL 或扫描 QrcodeURL 完成授权。
//
// appKey: 应用的 AppKey (即 client_id)
//
// 文档: https://pan.baidu.com/union/doc/al0rwqzzl
func (s *AuthService) DeviceCode(ctx context.Context, appKey string) (*DeviceCodeResponse, error) {
	if appKey == "" {
		return nil, fmt.Errorf("baidupan: DeviceCode appKey must not be empty")
	}

	q := url.Values{}
	q.Set("response_type", "device_code")
	q.Set("client_id", appKey)
	q.Set("scope", "basic,netdisk")

	var resp DeviceCodeResponse
	_, err := s.client.doOAuthGet(ctx, "/oauth/2.0/device/code", q, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
