package api

import (
	"context"
	"encoding/json"
	"net/url"
)

// =============================================================================
// IoTQueryUInfo 查询用户身份信息及VIP权限
// 接口: /rest/2.0/xpan/device?method=iotqueryuinfo
// =============================================================================

// IoTQueryUInfoParams 是 IoTQueryUInfo 接口的请求参数。
type IoTQueryUInfoParams struct {
	// DeviceID 设备ID（必填）。
	DeviceID string
}

// IoTQueryUInfoResponse 是 IoTQueryUInfo 接口的响应。
type IoTQueryUInfoResponse struct {
	// RequestID 请求唯一标识（API 可能返回数字或字符串，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`

	// ErrorCode 错误码，0 表示成功。
	ErrorCode int `json:"error_code"`

	// ErrorMsg 错误信息。
	ErrorMsg string `json:"error_msg"`

	// Data 数据对象。
	Data *IoTQueryUInfoData `json:"data,omitempty"`
}

// IoTQueryUInfoData 是 IoTQueryUInfo 响应中的 data 对象。
type IoTQueryUInfoData struct {
	// HasPrivilege 是否有特权（0:无, 1:有），当用户是SVIP或IoT SVIP未过期时为1。
	HasPrivilege int `json:"has_privilege"`

	// IsSVIP 是否是SVIP（0:否, 1:是）。
	IsSVIP int `json:"is_svip"`

	// IsIoTSVIP 是否是IoT SVIP（0:否, 1:是）。
	IsIoTSVIP int `json:"is_iot_svip"`

	// StartTime IoT SVIP开始时间（Unix时间戳）。
	StartTime uint64 `json:"start_time"`

	// EndTime IoT SVIP结束时间（Unix时间戳）。
	EndTime uint64 `json:"end_time"`

	// Now 当前服务器时间（Unix时间戳）。
	Now int64 `json:"now"`
}

// IoTQueryUInfo 查询用户身份信息及VIP权限。
//
// 接口地址: GET https://pan.baidu.com/rest/2.0/xpan/device?method=iotqueryuinfo
//
// 请求示例:
//
//	resp, err := client.Nas.IoTQueryUInfo(ctx, &api.IoTQueryUInfoParams{
//	    DeviceID: "your_device_id",
//	})
func (s *NasService) IoTQueryUInfo(ctx context.Context, params *IoTQueryUInfoParams) (*IoTQueryUInfoResponse, error) {
	q := url.Values{}
	q.Set("method", "iotqueryuinfo")

	if params != nil {
		if params.DeviceID != "" {
			q.Set("device_id", params.DeviceID)
		}
	}

	var resp IoTQueryUInfoResponse
	_, err := s.client.doGet(ctx, "/rest/2.0/xpan/device", q, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
