package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// =============================================================================
// CreateFile 创建文件接口（上传流程第三步）
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
// =============================================================================

// CreateFileParams 是 CreateFile 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
type CreateFileParams struct {
	// Path 上传后使用的文件绝对路径（必填）。
	Path string

	// Size 文件或目录的大小（byte），必填。
	Size int64

	// UploadID 预创建返回的 uploadid（必填）。
	UploadID string

	// BlockList 文件各分片 MD5 数组（必填）。
	// 应为上传分片后服务端返回的 MD5 列表。
	BlockList []string

	// IsDir 是否为目录（选填），0=文件，1=目录。默认 0。
	IsDir *int

	// RType 文件命名策略（选填）:
	//   0=不重命名（默认），1=路径冲突自动重命名，2=路径冲突覆盖，3=路径冲突自动重命名。
	RType *int

	// IsRevision 是否开启版本管理（选填），1=开启。
	IsRevision *int
}

// CreateFileResponse 是 CreateFile 接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
type CreateFileResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// FsID 文件 ID。
	FsID int64 `json:"fs_id"`

	// Path 文件路径。
	Path string `json:"path"`

	// ServerFilename 文件名。
	ServerFilename string `json:"server_filename"`

	// Size 文件大小（byte）。
	Size int64 `json:"size"`

	// Ctime 创建时间（Unix 时间戳）。
	Ctime int64 `json:"ctime"`

	// Mtime 修改时间（Unix 时间戳）。
	Mtime int64 `json:"mtime"`

	// MD5 文件 MD5。
	MD5 string `json:"md5"`

	// Isdir 是否为目录（0-否，1-是）。
	Isdir int `json:"isdir"`

	// Category 文件类型。
	Category int `json:"category"`
}

// CreateFile 创建文件，合并已上传的分片。
//
// 接口地址: POST https://pan.baidu.com/rest/2.0/xpan/file?method=create
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
//
// 请求示例:
//
//	resp, err := client.Upload.CreateFile(ctx, &api.CreateFileParams{
//	    Path:      "/apps/myapp/test.txt",
//	    Size:      1024,
//	    UploadID:  "N1-MTAu...",
//	    BlockList: []string{"ab56b4d92b40713acc5af89985d4b786"},
//	})
func (s *UploadService) CreateFile(ctx context.Context, params *CreateFileParams) (*CreateFileResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: CreateFile params must not be nil")
	}
	if params.Path == "" {
		return nil, fmt.Errorf("baidupan: CreateFile path must not be empty")
	}
	if params.UploadID == "" {
		return nil, fmt.Errorf("baidupan: CreateFile uploadid must not be empty")
	}
	if len(params.BlockList) == 0 {
		return nil, fmt.Errorf("baidupan: CreateFile block_list must not be empty")
	}

	q := url.Values{}
	q.Set("method", "create")

	body := url.Values{}
	body.Set("path", params.Path)
	body.Set("size", strconv.FormatInt(params.Size, 10))
	body.Set("uploadid", params.UploadID)

	isdir := 0
	if params.IsDir != nil {
		isdir = *params.IsDir
	}
	body.Set("isdir", strconv.Itoa(isdir))

	// block_list 序列化为 JSON 数组字符串
	blockListJSON, err := json.Marshal(params.BlockList)
	if err != nil {
		return nil, fmt.Errorf("baidupan: marshal block_list: %w", err)
	}
	body.Set("block_list", string(blockListJSON))

	if params.RType != nil {
		body.Set("rtype", strconv.Itoa(*params.RType))
	}
	if params.IsRevision != nil {
		body.Set("is_revision", strconv.Itoa(*params.IsRevision))
	}

	var resp CreateFileResponse
	_, err = s.client.doPost(ctx, "/rest/2.0/xpan/file", q, body, &resp)
	if err != nil {
		return nil, fmt.Errorf("upload create: %w", err)
	}
	return &resp, nil
}
