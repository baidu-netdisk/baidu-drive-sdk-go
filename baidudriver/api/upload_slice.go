package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
)

// =============================================================================
// SliceUpload 分片上传接口
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
// =============================================================================

// SliceUploadParams 是 SliceUpload 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
type SliceUploadParams struct {
	// Path 上传后使用的文件绝对路径（必填）。
	Path string

	// UploadID 预创建返回的 uploadid（必填）。
	UploadID string

	// PartSeq 文件分片序号，从 0 开始（必填）。
	PartSeq int

	// File 分片数据（必填）。
	File io.Reader
}

// SliceUploadResponse 是 SliceUpload 接口的响应。
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
type SliceUploadResponse struct {
	// Errno 错误码，0 表示成功。
	Errno int `json:"errno"`

	// MD5 上传分片的 MD5 值。
	MD5 string `json:"md5"`

	// RequestID 请求唯一标识（API 可能返回数字或字符串，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`
}

// SliceUpload 上传文件分片。
//
// 接口地址: POST https://d.pcs.baidu.com/rest/2.0/pcs/superfile2?method=upload
//
// 使用 multipart/form-data 编码上传文件分片数据，通过 io.Pipe 流式传输
// 避免将整个分片缓存在内存中。
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
//
// 请求示例:
//
//	resp, err := client.Upload.SliceUpload(ctx, &api.SliceUploadParams{
//	    Path:     "/apps/myapp/test.txt",
//	    UploadID: "N1-MTAu...",
//	    PartSeq:  0,
//	    File:     file,
//	})
func (s *UploadService) SliceUpload(ctx context.Context, params *SliceUploadParams) (*SliceUploadResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: SliceUpload params must not be nil")
	}
	if params.Path == "" {
		return nil, fmt.Errorf("baidupan: SliceUpload path must not be empty")
	}
	if params.UploadID == "" {
		return nil, fmt.Errorf("baidupan: SliceUpload uploadid must not be empty")
	}
	if params.File == nil {
		return nil, fmt.Errorf("baidupan: SliceUpload file must not be nil")
	}

	q := url.Values{}
	q.Set("method", "upload")
	q.Set("type", "tmpfile")
	q.Set("path", params.Path)
	q.Set("uploadid", params.UploadID)
	q.Set("partseq", strconv.Itoa(params.PartSeq))

	var resp SliceUploadResponse
	_, err := s.client.doPostMultipart(ctx, s.client.pcsBaseURL, "/rest/2.0/pcs/superfile2", q, "file", "file", params.File, &resp)
	if err != nil {
		return nil, fmt.Errorf("upload slice: %w", err)
	}
	return &resp, nil
}
