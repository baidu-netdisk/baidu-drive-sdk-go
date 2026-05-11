package scene

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

const (
	// DefaultSliceSize 默认分片大小 4MB。
	DefaultSliceSize = 4 * 1024 * 1024

	// defaultMaxRetries 默认最大重试次数。
	defaultMaxRetries = 3
)

// UploadFileParams 是 UploadFile 的请求参数。
type UploadFileParams struct {
	// LocalPath 本地文件路径（必填）。
	LocalPath string

	// RemotePath 上传目标路径（必填）。
	RemotePath string

	// SliceSize 分片大小（选填），默认 4MB。
	SliceSize int64

	// RType 文件命名策略（选填）。
	RType *int
}

// UploadFileResult 是 UploadFile 的返回结果。
type UploadFileResult struct {
	FsID int64
	Path string
	MD5  string
	Size int64
}

// UploadFile 上传本地文件到百度网盘。
//
// 自动执行三步流程：Precreate → SliceUpload（分片上传）→ CreateFile。
// 每一步失败时使用 exponential backoff 重试（最多 3 次，间隔 1/2/4s）。
// 如果 Precreate 返回 return_type=2（秒传），则跳过后续步骤。
//
// 示例:
//
//	result, err := sc.UploadFile(ctx, &scene.UploadFileParams{
//	    LocalPath:  "/tmp/test.txt",
//	    RemotePath: "/apps/myapp/test.txt",
//	})
func (s *Scene) UploadFile(ctx context.Context, params *UploadFileParams) (*UploadFileResult, error) {
	if params == nil {
		return nil, fmt.Errorf("scene: UploadFile params must not be nil")
	}
	if params.LocalPath == "" {
		return nil, fmt.Errorf("scene: UploadFile local path must not be empty")
	}
	if params.RemotePath == "" {
		return nil, fmt.Errorf("scene: UploadFile remote path must not be empty")
	}

	sliceSize := params.SliceSize
	if sliceSize <= 0 {
		sliceSize = DefaultSliceSize
	}

	// 打开文件获取大小
	f, err := os.Open(params.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("scene: open file: %w", err)
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("scene: stat file: %w", err)
	}
	fileSize := fi.Size()

	// 计算分片 MD5 列表
	blockMD5s, err := computeBlockMD5List(f, sliceSize)
	if err != nil {
		return nil, fmt.Errorf("scene: compute block md5: %w", err)
	}

	// Step 1: Precreate（带重试）
	var precreateResp *api.PrecreateResponse
	err = retry(ctx, defaultMaxRetries, func() error {
		var e error
		precreateResp, e = s.client.Upload.Precreate(ctx, &api.PrecreateParams{
			Path:      params.RemotePath,
			Size:      fileSize,
			BlockList: blockMD5s,
			RType:     params.RType,
		})
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("scene: precreate: %w", err)
	}

	// 秒传成功
	if precreateResp.ReturnType == 2 {
		return &UploadFileResult{
			Path: params.RemotePath,
		}, nil
	}

	// Step 2: SliceUpload（带重试）
	sliceMD5s := make([]string, 0, len(precreateResp.BlockList))
	for _, idx := range precreateResp.BlockList {
		offset := int64(idx) * sliceSize
		size := sliceSize
		if offset+size > fileSize {
			size = fileSize - offset
		}

		var sliceResp *api.SliceUploadResponse
		err = retry(ctx, defaultMaxRetries, func() error {
			// Seek to slice start for each retry
			if _, err := f.Seek(offset, io.SeekStart); err != nil {
				return fmt.Errorf("seek to offset %d: %w", offset, err)
			}
			reader := io.LimitReader(f, size)

			var e error
			sliceResp, e = s.client.Upload.SliceUpload(ctx, &api.SliceUploadParams{
				Path:     params.RemotePath,
				UploadID: precreateResp.UploadID,
				PartSeq:  idx,
				File:     reader,
			})
			return e
		})
		if err != nil {
			return nil, fmt.Errorf("scene: slice upload part %d: %w", idx, err)
		}
		sliceMD5s = append(sliceMD5s, sliceResp.MD5)
	}

	// Step 3: CreateFile（带重试）
	var createResp *api.CreateFileResponse
	err = retry(ctx, defaultMaxRetries, func() error {
		var e error
		createResp, e = s.client.Upload.CreateFile(ctx, &api.CreateFileParams{
			Path:      params.RemotePath,
			Size:      fileSize,
			UploadID:  precreateResp.UploadID,
			BlockList: sliceMD5s,
			RType:     params.RType,
		})
		return e
	})
	if err != nil {
		return nil, fmt.Errorf("scene: create file: %w", err)
	}

	return &UploadFileResult{
		FsID: createResp.FsID,
		Path: createResp.Path,
		MD5:  createResp.MD5,
		Size: createResp.Size,
	}, nil
}

// computeBlockMD5List 计算文件各分片的 MD5。
func computeBlockMD5List(f *os.File, sliceSize int64) ([]string, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var md5s []string
	buf := make([]byte, sliceSize)
	for {
		n, err := io.ReadFull(f, buf)
		if n > 0 {
			h := md5.Sum(buf[:n])
			md5s = append(md5s, hex.EncodeToString(h[:]))
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	// Reset file position for subsequent reads
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return md5s, nil
}
