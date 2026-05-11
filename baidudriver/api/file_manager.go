package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// =============================================================================
// Layer 1: 文件管理 API 调用层
// 管理文件文档: https://pan.baidu.com/union/doc/mksg0s9l4
// 创建文件夹文档: https://pan.baidu.com/union/doc/6lbaqe1lw
// =============================================================================

// FileManagerService 处理文件管理相关操作（复制、移动、重命名、删除、创建文件夹）。
type FileManagerService service

// CopyMoveItem 复制/移动操作的文件项。
//
// 用于 Copy 和 Move 方法的 filelist 参数。
type CopyMoveItem struct {
	// Path 源文件路径。
	Path string `json:"path"`

	// Dest 目标目录路径。
	Dest string `json:"dest"`

	// Newname 目标文件名。
	Newname string `json:"newname"`

	// Ondup 文件级重复处理策略（选填）。
	// 优先级高于全局 ondup 参数。
	// 可选值: "fail"（返回失败）, "newcopy"（重命名）, "overwrite"（覆盖）, "skip"（跳过）。
	Ondup string `json:"ondup,omitempty"`
}

// RenameItem 重命名操作的文件项。
//
// 用于 Rename 方法的 filelist 参数。
type RenameItem struct {
	// Path 源文件路径。
	Path string `json:"path"`

	// Newname 新文件名。
	Newname string `json:"newname"`
}

// FileManagerResponse 管理文件接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/mksg0s9l4
type FileManagerResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// Info 每个文件的操作结果。
	Info []*FileManagerInfo `json:"info"`

	// TaskID 异步任务 ID，当 async=2 时返回。
	TaskID int64 `json:"taskid"`

	// RequestID 请求唯一标识（API 可能返回数字或字符串，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`
}

// FileManagerInfo 单个文件的操作结果。
type FileManagerInfo struct {
	// Errno 单个文件操作错误码。
	Errno int `json:"errno"`

	// Path 文件路径。
	Path string `json:"path"`
}

// doManage 是管理文件接口的内部实现。
func (s *FileManagerService) doManage(ctx context.Context, opera string, async int, ondup *string, filelist any) (*FileManagerResponse, error) {
	if filelist == nil {
		return nil, fmt.Errorf("baidupan: filemanager %s filelist must not be nil", opera)
	}

	filelistJSON, err := json.Marshal(filelist)
	if err != nil {
		return nil, fmt.Errorf("baidupan: marshal filelist: %w", err)
	}

	q := url.Values{}
	q.Set("method", "filemanager")
	q.Set("opera", opera)

	body := url.Values{}
	body.Set("async", strconv.Itoa(async))
	body.Set("filelist", string(filelistJSON))
	if ondup != nil {
		body.Set("ondup", *ondup)
	}

	var resp FileManagerResponse
	_, err = s.client.doPost(ctx, "/rest/2.0/xpan/file", q, body, &resp)
	if err != nil {
		return nil, fmt.Errorf("filemanager %s: %w", opera, err)
	}
	return &resp, nil
}
