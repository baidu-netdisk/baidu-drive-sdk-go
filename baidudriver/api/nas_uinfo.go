package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// =============================================================================
// UInfo 获取用户信息接口
// 文档: https://pan.baidu.com/union/doc/pksg0s9ns
// =============================================================================

// UInfoParams 是 UInfo 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/pksg0s9ns
type UInfoParams struct {
	// VipVersion 可选，设为 "v2" 返回准确会员身份。
	VipVersion *string
}

// UInfoResponse 是 UInfo 接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/pksg0s9ns
type UInfoResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// Errmsg 错误信息。
	Errmsg string `json:"errmsg"`

	// RequestID 请求唯一标识（API 可能返回数字或字符串，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`

	// BaiduName 百度账号。
	BaiduName string `json:"baidu_name"`

	// NetdiskName 网盘账号。
	NetdiskName string `json:"netdisk_name"`

	// AvatarURL 头像地址。
	AvatarURL string `json:"avatar_url"`

	// VipType 会员类型（0=普通用户，1=普通会员，2=超级会员）。
	VipType int `json:"vip_type"`

	// UK 用户 ID。
	UK int64 `json:"uk"`
}

// UInfo 获取用户基本信息。
//
// 接口地址: GET https://pan.baidu.com/rest/2.0/xpan/nas?method=uinfo
//
// 文档: https://pan.baidu.com/union/doc/pksg0s9ns
//
// 请求示例:
//
//	resp, err := client.Nas.UInfo(ctx, &api.UInfoParams{
//	    VipVersion: api.Ptr("v2"),
//	})
func (s *NasService) UInfo(ctx context.Context, params *UInfoParams) (*UInfoResponse, error) {
	q := url.Values{}
	q.Set("method", "uinfo")

	if params != nil {
		if params.VipVersion != nil {
			q.Set("vip_version", *params.VipVersion)
		}
	}

	var resp UInfoResponse
	_, err := s.client.doGet(ctx, "/rest/2.0/xpan/nas", q, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
