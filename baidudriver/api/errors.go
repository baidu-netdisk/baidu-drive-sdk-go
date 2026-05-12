package api

import (
	"errors"
	"fmt"
	"net/http"
)

// Common errno values from Baidu API.
const (
	ErrnoSuccess          = 0
	ErrnoUnknown          = -1
	ErrnoAccessDenied     = -6
	ErrnoFileNameIllegal  = -7
	ErrnoFileAlreadyExist = -8
	ErrnoPathNotExist     = -9
	ErrnoSpaceFull        = -10
	ErrnoParamError       = 2
	ErrnoAsyncTaskRunning = 111
	ErrnoLimitExceeded    = 31034
	ErrnoHitBlacklist     = 31023
	ErrnoSpaceNotEnough   = 31190
	// ErrnoHTTPError 表示 HTTP 状态码异常但 JSON 中无业务错误码的兜底错误。
	// 不与百度 API 的 errno 值域冲突。
	ErrnoHTTPError        = -9999
)

// APIError is returned when the Baidu API returns a non-zero errno.
type APIError struct {
	Errno    int    `json:"errno"`
	Errmsg   string `json:"errmsg"`
	RequestID string `json:"request_id,omitempty"`
	Response *http.Response `json:"-"`

	// 诊断信息（帮助开发者排查问题）
	// Method 请求方法（GET/POST）。
	Method string `json:"method,omitempty"`
	// URL 请求 URL（access_token 已脱敏）。
	URL string `json:"url,omitempty"`
	// ResponseBody 响应体摘要（截断至 1024 字节）。
	ResponseBody string `json:"response_body,omitempty"`
}

func (e *APIError) Error() string {
	if e.Errmsg != "" {
		return fmt.Sprintf("baidupan: API error errno=%d msg=%s", e.Errno, e.Errmsg)
	}
	return fmt.Sprintf("baidupan: API error errno=%d", e.Errno)
}

// IsErrno checks if an error is an APIError with the given errno.
func IsErrno(err error, errno int) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Errno == errno
	}
	return false
}
