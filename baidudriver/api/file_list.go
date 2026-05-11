package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// =============================================================================
// List 获取文件列表接口
// 文档: https://pan.baidu.com/union/doc/nksg0sat9
// =============================================================================

// ListParams 是 List 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/nksg0sat9
type ListParams struct {
	// Dir 目录路径（必填），默认 "/"。
	Dir string

	// Order 排序字段（选填）: name/time/size。
	Order *string

	// Desc 是否降序（选填），1=降序，0=升序。
	Desc *int

	// Start 起始位置（选填），默认 0。
	Start *int

	// Limit 返回数量（选填），默认 1000。
	Limit *int

	// Web 是否返回缩略图（选填），1=返回。
	Web *int

	// Folder 是否仅返回文件夹（选填），1=仅文件夹。
	Folder *int

	// Showempty 是否返回 dir_empty 属性（选填），1=返回。
	Showempty *int
}

// ListFileInfo 是 List 返回的单个文件信息。
//
// 文档: https://pan.baidu.com/union/doc/nksg0sat9
type ListFileInfo struct {
	// FsID 文件 ID。
	FsID int64 `json:"fs_id"`

	// Path 文件路径。
	Path string `json:"path"`

	// ServerFilename 文件名。
	ServerFilename string `json:"server_filename"`

	// Size 文件大小（byte）。
	Size int64 `json:"size"`

	// ServerMtime 服务器修改时间（Unix 时间戳）。
	ServerMtime int64 `json:"server_mtime"`

	// ServerCtime 服务器创建时间（Unix 时间戳）。
	ServerCtime int64 `json:"server_ctime"`

	// LocalMtime 本地修改时间（Unix 时间戳）。
	LocalMtime int64 `json:"local_mtime"`

	// LocalCtime 本地创建时间（Unix 时间戳）。
	LocalCtime int64 `json:"local_ctime"`

	// Isdir 是否为文件夹（0-否，1-是）。
	Isdir int `json:"isdir"`

	// Category 文件类型。
	Category int `json:"category"`

	// MD5 文件 MD5。
	MD5 string `json:"md5"`

	// DirEmpty 目录是否为空（仅 showempty=1 时返回）。
	DirEmpty *int `json:"dir_empty,omitempty"`

	// Thumbs 缩略图 URL（仅 web=1 时返回）。
	Thumbs map[string]string `json:"thumbs,omitempty"`
}

// ListResponse 是 List 接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/nksg0sat9
type ListResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// List 文件列表。
	List []*ListFileInfo `json:"list"`

	// RequestID 请求唯一标识（API 可能返回数字或字符串，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`

	// GUID 全局唯一标识。
	GUID int64 `json:"guid"`
}

// List 获取指定目录下的文件列表。
//
// 接口地址: GET https://pan.baidu.com/rest/2.0/xpan/file?method=list
//
// 文档: https://pan.baidu.com/union/doc/nksg0sat9
//
// 请求示例:
//
//	resp, err := client.File.List(ctx, &api.ListParams{
//	    Dir:   "/test",
//	    Limit: api.Ptr(100),
//	})
func (s *FileService) List(ctx context.Context, params *ListParams) (*ListResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: List params must not be nil")
	}
	if params.Dir == "" {
		params.Dir = "/"
	}

	q := url.Values{}
	q.Set("method", "list")
	q.Set("dir", params.Dir)
	if params.Order != nil {
		q.Set("order", *params.Order)
	}
	if params.Desc != nil {
		q.Set("desc", strconv.Itoa(*params.Desc))
	}
	if params.Start != nil {
		q.Set("start", strconv.Itoa(*params.Start))
	}
	if params.Limit != nil {
		q.Set("limit", strconv.Itoa(*params.Limit))
		// 服务端要求 start 和 limit 配合使用，单传 limit 不传 start 时 limit 会被忽略。
		if params.Start == nil {
			q.Set("start", "0")
		}
	}
	if params.Web != nil {
		q.Set("web", strconv.Itoa(*params.Web))
	}
	if params.Folder != nil {
		q.Set("folder", strconv.Itoa(*params.Folder))
	}
	if params.Showempty != nil {
		q.Set("showempty", strconv.Itoa(*params.Showempty))
	}

	var resp ListResponse
	_, err := s.client.doGet(ctx, "/rest/2.0/xpan/file", q, &resp)
	if err != nil {
		return nil, fmt.Errorf("file list: %w", err)
	}
	return &resp, nil
}
