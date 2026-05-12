package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// =============================================================================
// Precreate 预创建文件接口
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
// =============================================================================

// PrecreateParams 是 Precreate 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
type PrecreateParams struct {
	// Path 上传后使用的文件绝对路径（必填）。
	Path string

	// Size 文件或目录的大小（byte），必填。
	Size int64

	// BlockList 文件各分片 MD5 数组（必填）。
	// 文件被分为固定大小的分片后，对每个分片计算 MD5。
	BlockList []string

	// IsDir 是否为目录（选填），0=文件，1=目录。默认 0。
	IsDir *int

	// RType 文件命名策略（选填）:
	//   0=不重命名（默认），1=路径冲突自动重命名，2=路径冲突覆盖，3=路径冲突自动重命名。
	RType *int

	// Autoinit 固定值 1（选填）。默认会自动设置。
	Autoinit *int

	// IsRevision 是否开启版本管理（选填），1=开启。
	IsRevision *int
}

// PrecreateResponse 是 Precreate 接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
type PrecreateResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// RequestID 请求唯一标识（API 可能返回数字或字符串，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`

	// UploadID 上传 ID，用于后续分片上传和文件创建。
	// 秒传时为空字符串。
	UploadID string `json:"uploadid"`

	// BlockList 需要上传的分片序号列表。
	// 如果某个分片已存在服务端，则不在此列表中。
	BlockList []int `json:"block_list"`

	// ReturnType 返回类型:
	//   1=需要上传分片，2=秒传成功。
	ReturnType int `json:"return_type"`
}

// Precreate 预创建文件，获取上传所需的 uploadid。
//
// 接口地址: POST https://pan.baidu.com/rest/2.0/xpan/file?method=precreate
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
//
// 请求示例:
//
//	resp, err := client.Upload.Precreate(ctx, &api.PrecreateParams{
//	    Path:      "/apps/myapp/test.txt",
//	    Size:      1024,
//	    BlockList: []string{"ab56b4d92b40713acc5af89985d4b786"},
//	})
func (s *UploadService) Precreate(ctx context.Context, params *PrecreateParams) (*PrecreateResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: Precreate params must not be nil")
	}
	if params.Path == "" {
		return nil, fmt.Errorf("baidupan: Precreate path must not be empty")
	}
	if len(params.BlockList) == 0 {
		return nil, fmt.Errorf("baidupan: Precreate block_list must not be empty")
	}

	q := url.Values{}
	q.Set("method", "precreate")

	body := url.Values{}
	body.Set("path", params.Path)
	body.Set("size", strconv.FormatInt(params.Size, 10))
	body.Set("autoinit", "1")
	if params.Autoinit != nil {
		body.Set("autoinit", strconv.Itoa(*params.Autoinit))
	}

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

	var resp PrecreateResponse
	_, err = s.client.doPost(ctx, "/rest/2.0/xpan/file", q, body, &resp)
	if err != nil {
		return nil, fmt.Errorf("upload precreate: %w", err)
	}
	return &resp, nil
}
