package api

import (
	"context"
	"fmt"
	"io"
)

// =============================================================================
// Download 下载文件接口（下载流程第二步）
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
// =============================================================================

// DownloadParams 是 Download 接口的请求参数。
//
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
type DownloadParams struct {
	// Dlink 从 Meta 接口获取的下载链接（必填）。
	Dlink string
}

// Download 通过 dlink 下载文件内容。
//
// 返回文件内容流（io.ReadCloser）和 Content-Length。
// 调用方必须在使用完毕后关闭返回的 io.ReadCloser。
//
// 注意: 百度网盘 CDN 要求 User-Agent 包含 "pan.baidu.com"，
// 本方法自动设置该 User-Agent。
//
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
//
// 请求示例:
//
//	body, size, err := client.Download.Download(ctx, &api.DownloadParams{
//	    Dlink: "https://d.pcs.baidu.com/file/xxxxx",
//	})
//	defer body.Close()
//	io.Copy(localFile, body)
func (s *DownloadService) Download(ctx context.Context, params *DownloadParams) (io.ReadCloser, int64, error) {
	if params == nil {
		return nil, 0, fmt.Errorf("baidupan: Download params must not be nil")
	}
	if params.Dlink == "" {
		return nil, 0, fmt.Errorf("baidupan: Download dlink must not be empty")
	}

	body, size, err := s.client.doGetStream(ctx, params.Dlink, "pan.baidu.com")
	if err != nil {
		return nil, 0, fmt.Errorf("download file: %w", err)
	}
	return body, size, nil
}
