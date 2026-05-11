package api

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
)

// =============================================================================
// 创建文件夹接口
// 文档: https://pan.baidu.com/union/doc/6lbaqe1lw
// =============================================================================

// MkdirParams 创建文件夹接口的请求参数。
//
// 接口地址: POST https://pan.baidu.com/rest/2.0/xpan/file?method=create
//
// 文档: https://pan.baidu.com/union/doc/6lbaqe1lw
type MkdirParams struct {
	// Path 文件夹的绝对路径（必填）。
	// 例如: "/apps/appName/mydir"
	Path string

	// Rtype 文件命名策略（选填）。
	// 0 - 不重命名，路径冲突时返回错误（默认值）
	// 1 - 路径冲突时自动重命名
	Rtype *int

	// LocalCtime 客户端创建时间（选填）。
	// Unix 时间戳（秒），默认为当前时间。
	LocalCtime *int64

	// LocalMtime 客户端修改时间（选填）。
	// Unix 时间戳（秒），默认为当前时间。
	LocalMtime *int64

	// Mode 上传方式（选填）。
	// 1=手动, 2=批量上传, 3=文件自动备份, 4=相册自动备份, 5=视频自动备份
	Mode *int
}

// MkdirResponse 创建文件夹接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/6lbaqe1lw
type MkdirResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// FsID 文件在云端的唯一标识 ID。
	FsID int64 `json:"fs_id"`

	// Category 分类类型，文件夹对应值为 6。
	Category int `json:"category"`

	// Path 创建后的文件夹绝对路径。
	Path string `json:"path"`

	// Ctime 文件创建时间（Unix 时间戳）。
	Ctime int64 `json:"ctime"`

	// Mtime 文件修改时间（Unix 时间戳）。
	Mtime int64 `json:"mtime"`

	// Isdir 是否为目录: 0=文件, 1=目录。
	Isdir int `json:"isdir"`

	// Status 状态码。
	Status int `json:"status"`
}

// Mkdir 创建文件夹。
//
// 在网盘中创建一个新的文件夹。
//
// API 文档: https://pan.baidu.com/union/doc/6lbaqe1lw
//
// 请求方式: POST https://pan.baidu.com/rest/2.0/xpan/file?method=create
//
// 示例:
//
//	resp, err := client.FileManager.Mkdir(ctx, &api.MkdirParams{
//	    Path:  "/apps/appName/mydir",
//	    Rtype: api.Ptr(1),
//	})
func (s *FileManagerService) Mkdir(ctx context.Context, params *MkdirParams) (*MkdirResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: Mkdir params must not be nil")
	}
	if params.Path == "" {
		return nil, fmt.Errorf("baidupan: Mkdir path must not be empty")
	}

	q := url.Values{}
	q.Set("method", "create")

	body := url.Values{}
	body.Set("path", params.Path)
	body.Set("isdir", "1")
	if params.Rtype != nil {
		body.Set("rtype", strconv.Itoa(*params.Rtype))
	}
	if params.LocalCtime != nil {
		body.Set("local_ctime", strconv.FormatInt(*params.LocalCtime, 10))
	}
	if params.LocalMtime != nil {
		body.Set("local_mtime", strconv.FormatInt(*params.LocalMtime, 10))
	}
	if params.Mode != nil {
		body.Set("mode", strconv.Itoa(*params.Mode))
	}

	var resp MkdirResponse
	_, err := s.client.doPost(ctx, "/rest/2.0/xpan/file", q, body, &resp)
	if err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return &resp, nil
}
