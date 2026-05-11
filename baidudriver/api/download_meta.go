package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// =============================================================================
// Meta 获取文件元信息接口（下载流程第一步）
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
// =============================================================================

// MetaParams 是 Meta 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
type MetaParams struct {
	// FsIDs 文件 ID 列表（必填），最多 100 个。
	FsIDs []int64

	// Extra 是否获取缩略图等额外信息（选填），1=是。
	Extra *int
}

// FileMeta 表示单个文件的元信息。
type FileMeta struct {
	// FsID 文件 ID。
	FsID int64 `json:"fs_id"`

	// Path 文件路径。
	Path string `json:"path"`

	// Filename 文件名。
	Filename string `json:"filename"`

	// Size 文件大小（byte）。
	Size int64 `json:"size"`

	// MD5 文件 MD5。
	MD5 string `json:"md5"`

	// Dlink 下载链接（有效期 8 小时）。
	Dlink string `json:"dlink"`

	// Isdir 是否为目录（0-否，1-是）。
	Isdir int `json:"isdir"`

	// Category 文件类型。
	Category int `json:"category"`

	// Thumbs 缩略图信息（需 extra=1）。
	Thumbs map[string]string `json:"thumbs,omitempty"`
}

// MetaResponse 是 Meta 接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
type MetaResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// List 文件元信息列表。
	List []FileMeta `json:"list"`
}

// Meta 获取文件元信息（含下载链接 dlink）。
//
// 接口地址: GET https://pan.baidu.com/rest/2.0/xpan/multimedia?method=filemetas
//
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
//
// 请求示例:
//
//	resp, err := client.Download.Meta(ctx, &api.MetaParams{
//	    FsIDs: []int64{123456789},
//	})
//	dlink := resp.List[0].Dlink
func (s *DownloadService) Meta(ctx context.Context, params *MetaParams) (*MetaResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: Meta params must not be nil")
	}
	if len(params.FsIDs) == 0 {
		return nil, fmt.Errorf("baidupan: Meta fsids must not be empty")
	}

	q := url.Values{}
	q.Set("method", "filemetas")
	q.Set("dlink", "1")

	// fsids 序列化为 JSON 数组字符串
	fsidsJSON, err := json.Marshal(params.FsIDs)
	if err != nil {
		return nil, fmt.Errorf("baidupan: marshal fsids: %w", err)
	}
	q.Set("fsids", string(fsidsJSON))

	if params.Extra != nil {
		q.Set("extra", strconv.Itoa(*params.Extra))
	}

	var resp MetaResponse
	_, err = s.client.doGet(ctx, "/rest/2.0/xpan/multimedia", q, &resp)
	if err != nil {
		return nil, fmt.Errorf("download meta: %w", err)
	}
	return &resp, nil
}
