package scene

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// DownloadFileParams 是 DownloadFile 的请求参数。
type DownloadFileParams struct {
	// FsID 文件 ID（必填）。
	FsID int64

	// LocalPath 下载保存的本地路径（必填）。
	LocalPath string
}

// DownloadFileResult 是 DownloadFile 的返回结果。
type DownloadFileResult struct {
	Path     string
	Size     int64
	MD5      string
	Filename string
}

// DownloadFile 从百度网盘下载文件到本地。
//
// 自动执行两步流程：Meta（获取 dlink）→ Download（流式下载）。
// 每一步失败时使用 exponential backoff 重试（最多 3 次，间隔 1/2/4s）。
//
// 示例:
//
//	result, err := sc.DownloadFile(ctx, &scene.DownloadFileParams{
//	    FsID:      123456789,
//	    LocalPath: "/tmp/downloaded.txt",
//	})
func (s *Scene) DownloadFile(ctx context.Context, params *DownloadFileParams) (*DownloadFileResult, error) {
	if params == nil {
		return nil, fmt.Errorf("scene: DownloadFile params must not be nil")
	}
	if params.FsID == 0 {
		return nil, fmt.Errorf("scene: DownloadFile fsid must not be zero")
	}
	if params.LocalPath == "" {
		return nil, fmt.Errorf("scene: DownloadFile local path must not be empty")
	}

	// Step 1: Meta — 获取 dlink（带重试）
	var metaResp *api.MetaResponse
	err := retry(ctx, defaultMaxRetries, func() error {
		var e error
		metaResp, e = s.client.Download.Meta(ctx, &api.MetaParams{
			FsIDs: []int64{params.FsID},
		})
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("scene: download meta: %w", err)
	}

	if len(metaResp.List) == 0 {
		return nil, fmt.Errorf("scene: download meta returned empty list for fsid %d", params.FsID)
	}
	fileMeta := metaResp.List[0]
	if fileMeta.Dlink == "" {
		return nil, fmt.Errorf("scene: download meta returned empty dlink for fsid %d", params.FsID)
	}

	// Step 2: Download — 流式下载到本地文件（带重试）
	var body io.ReadCloser
	var contentLength int64
	err = retry(ctx, defaultMaxRetries, func() error {
		var e error
		body, contentLength, e = s.client.Download.Download(ctx, &api.DownloadParams{
			Dlink: fileMeta.Dlink,
		})
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("scene: download file: %w", err)
	}
	defer body.Close()

	// 写入本地文件
	outFile, err := os.Create(params.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("scene: create output file: %w", err)
	}

	written, err := io.Copy(outFile, body)
	outFile.Close()
	if err != nil {
		os.Remove(params.LocalPath) // 清理残留的部分文件
		return nil, fmt.Errorf("scene: write file: %w", err)
	}

	size := contentLength
	if size <= 0 {
		size = written
	}

	return &DownloadFileResult{
		Path:     fileMeta.Path,
		Size:     size,
		MD5:      fileMeta.MD5,
		Filename: fileMeta.Filename,
	}, nil
}
