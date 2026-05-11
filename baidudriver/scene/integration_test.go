package scene

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// 集成测试 — 需要真实 access_token。
// 运行方式:
//   BDPAN_ACCESS_TOKEN=xxx go test -tags integration -run TestIntegration -v ./baidupan/scene/

func getTestToken(t *testing.T) string {
	t.Helper()
	token := os.Getenv("BDPAN_ACCESS_TOKEN")
	if token == "" {
		t.Skip("BDPAN_ACCESS_TOKEN not set, skipping integration test")
	}
	return token
}

func TestIntegration_UploadDownloadVerify(t *testing.T) {
	token := getTestToken(t)

	// 生成随机文件内容（1KB）
	content := make([]byte, 1024)
	if _, err := rand.Read(content); err != nil {
		t.Fatalf("generate random content: %v", err)
	}

	// 计算原始 MD5
	h := md5.Sum(content)
	originalMD5 := hex.EncodeToString(h[:])

	// 写入临时文件
	uploadFile, err := os.CreateTemp("", "integration_upload_*")
	if err != nil {
		t.Fatalf("create upload temp: %v", err)
	}
	if _, err := uploadFile.Write(content); err != nil {
		uploadFile.Close()
		t.Fatalf("write upload temp: %v", err)
	}
	uploadFile.Close()
	defer os.Remove(uploadFile.Name())

	remotePath := fmt.Sprintf("/apps/bdpan_sdk_test/integration_test_%s.bin", originalMD5[:8])

	c := api.NewClient(api.WithAccessToken(token))
	sc := New(c)
	ctx := context.Background()

	// Step 1: 上传
	t.Logf("uploading to %s...", remotePath)
	uploadResult, err := sc.UploadFile(ctx, &UploadFileParams{
		LocalPath:  uploadFile.Name(),
		RemotePath: remotePath,
		RType:      api.Ptr(1), // 自动重命名防冲突
	})
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	t.Logf("upload success: fsid=%d, path=%s, md5=%s", uploadResult.FsID, uploadResult.Path, uploadResult.MD5)

	if uploadResult.FsID == 0 {
		t.Fatal("upload returned fsid=0")
	}

	// Step 2: 获取 Meta 验证
	t.Logf("fetching meta for fsid=%d...", uploadResult.FsID)
	metaResp, err := c.Download.Meta(ctx, &api.MetaParams{
		FsIDs: []int64{uploadResult.FsID},
	})
	if err != nil {
		t.Fatalf("meta failed: %v", err)
	}
	if len(metaResp.List) == 0 {
		t.Fatal("meta returned empty list")
	}
	meta := metaResp.List[0]
	t.Logf("meta: size=%d, md5=%s, dlink=%s", meta.Size, meta.MD5, meta.Dlink[:50]+"...")

	if meta.Size != int64(len(content)) {
		t.Errorf("meta size = %d, want %d", meta.Size, len(content))
	}

	// Step 3: 下载
	downloadFile, err := os.CreateTemp("", "integration_download_*")
	if err != nil {
		t.Fatalf("create download temp: %v", err)
	}
	downloadFile.Close()
	defer os.Remove(downloadFile.Name())

	t.Logf("downloading to %s...", downloadFile.Name())
	downloadResult, err := sc.DownloadFile(ctx, &DownloadFileParams{
		FsID:      uploadResult.FsID,
		LocalPath: downloadFile.Name(),
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	t.Logf("download success: size=%d, path=%s", downloadResult.Size, downloadResult.Path)

	// Step 4: 校验 MD5
	downloadedContent, err := os.ReadFile(downloadFile.Name())
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}

	dh := md5.Sum(downloadedContent)
	downloadedMD5 := hex.EncodeToString(dh[:])

	if downloadedMD5 != originalMD5 {
		t.Errorf("MD5 mismatch: original=%s, downloaded=%s", originalMD5, downloadedMD5)
	} else {
		t.Logf("MD5 verification PASSED: %s", originalMD5)
	}

	if !bytes.Equal(content, downloadedContent) {
		t.Errorf("content mismatch: original len=%d, downloaded len=%d", len(content), len(downloadedContent))
	}

	// Step 5: 清理 — 删除测试文件
	t.Logf("cleaning up %s...", uploadResult.Path)
	err = sc.DeleteFile(ctx, []string{uploadResult.Path})
	if err != nil {
		t.Logf("cleanup warning (non-fatal): %v", err)
	}

	t.Log("Integration test PASSED")
}

func TestIntegration_CopyFile(t *testing.T) {
	token := getTestToken(t)

	src := os.Getenv("BDPAN_COPY_SRC")
	destDir := os.Getenv("BDPAN_COPY_DEST_DIR")
	newname := os.Getenv("BDPAN_COPY_NEWNAME")
	if src == "" || destDir == "" || newname == "" {
		t.Skip("BDPAN_COPY_SRC / BDPAN_COPY_DEST_DIR / BDPAN_COPY_NEWNAME not set, skipping")
	}

	c := api.NewClient(api.WithAccessToken(token))
	sc := New(c)
	ctx := context.Background()

	t.Logf("copying %s → %s/%s", src, destDir, newname)
	err := sc.CopyFile(ctx, src, destDir, newname)
	if err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}
	t.Logf("CopyFile PASSED")
}

func TestIntegration_CopyFileWithOndup(t *testing.T) {
	token := getTestToken(t)

	src := os.Getenv("BDPAN_COPY_SRC")
	destDir := os.Getenv("BDPAN_COPY_DEST_DIR")
	newname := os.Getenv("BDPAN_COPY_NEWNAME")
	ondup := os.Getenv("BDPAN_COPY_ONDUP")
	if src == "" || destDir == "" || newname == "" || ondup == "" {
		t.Skip("BDPAN_COPY_SRC / BDPAN_COPY_DEST_DIR / BDPAN_COPY_NEWNAME / BDPAN_COPY_ONDUP not set, skipping")
	}

	c := api.NewClient(api.WithAccessToken(token), api.WithDebug(true), api.WithLogger(os.Stderr))
	sc := New(c)
	ctx := context.Background()

	t.Logf("copying %s → %s/%s (ondup=%s)", src, destDir, newname, ondup)
	err := sc.CopyFileWithOndup(ctx, src, destDir, newname, ondup)
	if err != nil {
		t.Fatalf("CopyFileWithOndup failed: %v", err)
	}
	t.Logf("CopyFileWithOndup PASSED")
}

func TestIntegration_MoveFile(t *testing.T) {
	token := getTestToken(t)

	src := os.Getenv("BDPAN_MOVE_SRC")
	destDir := os.Getenv("BDPAN_MOVE_DEST_DIR")
	newname := os.Getenv("BDPAN_MOVE_NEWNAME")
	if src == "" || destDir == "" || newname == "" {
		t.Skip("BDPAN_MOVE_SRC / BDPAN_MOVE_DEST_DIR / BDPAN_MOVE_NEWNAME not set, skipping")
	}

	c := api.NewClient(api.WithAccessToken(token), api.WithDebug(true), api.WithLogger(os.Stderr))
	sc := New(c)
	ctx := context.Background()

	t.Logf("moving %s → %s/%s", src, destDir, newname)
	err := sc.MoveFile(ctx, src, destDir, newname)
	if err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}
	t.Logf("MoveFile PASSED")
}

// Helper to suppress unused import
var _ = io.Discard
