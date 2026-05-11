package api

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// =============================================================================
// Quota 获取网盘容量信息接口
// 文档: https://pan.baidu.com/union/doc/Cksg0s9ic
// =============================================================================

// QuotaParams 是 Quota 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/Cksg0s9ic
type QuotaParams struct {
	// CheckFree 可选，是否检查免费信息，0为不查，1为查，默认为0。
	CheckFree *int

	// CheckExpire 可选，是否检查过期信息，0为不查，1为查，默认为0。
	CheckExpire *int
}

// QuotaResponse 是 Quota 接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/Cksg0s9ic
type QuotaResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// Errmsg 错误信息。
	Errmsg string `json:"errmsg"`

	// RequestID 请求唯一标识（API 可能返回数字或字符串，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`

	// Total 总空间大小，单位B。
	Total int64 `json:"total"`

	// Used 已使用大小，单位B。
	Used int64 `json:"used"`

	// Free 免费容量，单位B。
	Free int64 `json:"free"`

	// Expire 7天内是否有容量到期。
	Expire bool `json:"expire"`
}

// Quota 获取网盘容量信息。
//
// 接口地址: GET https://pan.baidu.com/api/quota
//
// 文档: https://pan.baidu.com/union/doc/Cksg0s9ic
//
// 请求示例:
//
//	resp, err := client.Nas.Quota(ctx, nil)
//	fmt.Printf("总容量: %d bytes, 已使用: %d bytes\n", resp.Total, resp.Used)
func (s *NasService) Quota(ctx context.Context, params *QuotaParams) (*QuotaResponse, error) {
	q := url.Values{}

	if params != nil {
		if params.CheckFree != nil {
			q.Set("checkfree", strconv.Itoa(*params.CheckFree))
		}
		if params.CheckExpire != nil {
			q.Set("checkexpire", strconv.Itoa(*params.CheckExpire))
		}
	}

	var resp QuotaResponse
	_, err := s.client.doGet(ctx, "/api/quota", q, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
