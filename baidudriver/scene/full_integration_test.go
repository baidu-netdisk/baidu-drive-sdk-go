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
	"path"
	"strconv"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

const maxIntegrationDownloadSize = 10 * 1024 * 1024

// TestIntegration_FullSDK exercises every public API and Scene capability that
// can be verified against a real account. Remote writes are isolated under a
// unique /apps/baidu_sdk_full_test_* directory and require an explicit gate.
func TestIntegration_FullSDK(t *testing.T) {
	token := getTestToken(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	client := api.NewClient(api.WithAccessToken(token))
	sc := New(client)

	var userUK int64
	var fileInfoFsID int64
	if raw := os.Getenv("BDPAN_FILE_INFO_FSID"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			t.Fatalf("BDPAN_FILE_INFO_FSID must be a positive integer: %q", raw)
		}
		fileInfoFsID = parsed
	}
	t.Run("API/Nas/UInfo", func(t *testing.T) {
		resp, err := client.Nas.UInfo(ctx, &api.UInfoParams{VipVersion: api.Ptr("v2")})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || resp.UK == 0 {
			t.Fatalf("unexpected uinfo response: errno=%d uk=%d", resp.Errno, resp.UK)
		}
		userUK = resp.UK
	})

	t.Run("API/Nas/Quota", func(t *testing.T) {
		resp, err := client.Nas.Quota(ctx, &api.QuotaParams{
			CheckFree:   api.Ptr(1),
			CheckExpire: api.Ptr(1),
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || resp.Total <= 0 || resp.Used < 0 || resp.Used > resp.Total {
			t.Fatalf("unexpected quota response: errno=%d total=%d used=%d", resp.Errno, resp.Total, resp.Used)
		}
	})

	t.Run("API/File/List", func(t *testing.T) {
		resp, err := client.File.List(ctx, &api.ListParams{
			Dir:       "/",
			Order:     api.Ptr("time"),
			Desc:      api.Ptr(1),
			Start:     api.Ptr(0),
			Limit:     api.Ptr(5),
			Web:       api.Ptr(0),
			Folder:    api.Ptr(0),
			Showempty: api.Ptr(1),
		})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.List) > 5 {
			t.Fatalf("unexpected list response: errno=%d count=%d", resp.Errno, len(resp.List))
		}
		for _, file := range resp.List {
			if file != nil && file.FsID > 0 && file.Isdir == 0 {
				fileInfoFsID = file.FsID
				break
			}
		}
	})

	// listall 仅允许访问当前应用授权目录；该目录已由本轮 curl 边界探测确认。
	fileInfoPath := "/apps/bdpan测试"
	t.Run("API/File/ListAll", func(t *testing.T) {
		resp, err := client.File.ListAll(ctx, &api.ListAllParams{Path: fileInfoPath, Recursion: api.Ptr(1), Start: api.Ptr(0), Limit: api.Ptr(5)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.List) > 5 {
			t.Fatalf("unexpected listall response: errno=%d count=%d", resp.Errno, len(resp.List))
		}
		if fileInfoFsID == 0 {
			for _, file := range resp.List {
				if file != nil && file.FsID > 0 && file.Isdir == 0 {
					fileInfoFsID = file.FsID
					break
				}
			}
		}
	})

	t.Run("API/File/DocumentList", func(t *testing.T) {
		resp, err := client.File.DocumentList(ctx, &api.DocumentListParams{ParentPath: fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(5)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.Info) > 5 {
			t.Fatalf("unexpected doclist response: errno=%d count=%d", resp.Errno, len(resp.Info))
		}
	})

	t.Run("API/File/ImageList", func(t *testing.T) {
		resp, err := client.File.ImageList(ctx, &api.ImageListParams{ParentPath: fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(5)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.Info) > 5 {
			t.Fatalf("unexpected imagelist response: errno=%d count=%d", resp.Errno, len(resp.Info))
		}
	})

	t.Run("API/File/VideoList", func(t *testing.T) {
		resp, err := client.File.VideoList(ctx, &api.VideoListParams{ParentPath: fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(5)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.Info) > 5 {
			t.Fatalf("unexpected videolist response: errno=%d count=%d", resp.Errno, len(resp.Info))
		}
	})

	t.Run("API/File/BTList", func(t *testing.T) {
		resp, err := client.File.BTList(ctx, &api.BTListParams{ParentPath: fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(5)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.Info) > 5 {
			t.Fatalf("unexpected btlist response: errno=%d count=%d", resp.Errno, len(resp.Info))
		}
	})

	t.Run("API/File/CategoryCount", func(t *testing.T) {
		resp, err := client.File.CategoryCount(ctx, &api.CategoryCountParams{Category: 4, ParentPath: fileInfoPath, Recursion: api.Ptr(0)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 {
			t.Fatalf("unexpected category count response: errno=%d", resp.Errno)
		}
	})

	t.Run("API/File/CategoryList", func(t *testing.T) {
		resp, err := client.File.CategoryList(ctx, &api.CategoryListParams{Category: "4", ParentPath: fileInfoPath, Recursion: api.Ptr(0), Start: api.Ptr(0), Limit: api.Ptr(5)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.List) > 5 {
			t.Fatalf("unexpected category list response: errno=%d count=%d", resp.Errno, len(resp.List))
		}
		if fileInfoFsID == 0 {
			for _, file := range resp.List {
				if file != nil && file.FsID > 0 && file.Isdir == 0 {
					fileInfoFsID = file.FsID
					break
				}
			}
		}
	})

	t.Run("API/File/QueryFileInfo", func(t *testing.T) {
		if fileInfoFsID == 0 {
			t.Skip("测试目录中没有可查询的文件")
		}
		resp, err := client.File.QueryFileInfo(ctx, &api.QueryFileInfoParams{FsIDs: []int64{fileInfoFsID}, Dlink: api.Ptr(0), Extra: api.Ptr(1)})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || len(resp.List) == 0 || resp.List[0].FsID != fileInfoFsID {
			t.Fatalf("unexpected filemetas response: errno=%d count=%d fsid=%d", resp.Errno, len(resp.List), fileInfoFsID)
		}
	})

	t.Run("API/File/KeywordSearch", func(t *testing.T) {
		resp, err := client.File.KeywordSearch(ctx, &api.KeywordSearchParams{Key: "README", Dir: fileInfoPath})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 {
			t.Fatalf("unexpected keyword search response: errno=%d", resp.Errno)
		}
	})

	var downloadable *api.UniSearchFileInfo
	t.Run("API/File/UniSearch", func(t *testing.T) {
		params := &api.UniSearchParams{
			Query:      "pdf",
			Scene:      "mcpserver",
			Category:   []int{4},
			Num:        api.Ptr(20),
			Stream:     api.Ptr(0),
			SearchType: api.Ptr(0),
		}
		if userUK != 0 {
			params.Dirs = []api.UniSearchDir{{UK: userUK, Path: "/"}}
		}
		resp, err := client.File.UniSearch(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		if resp.ErrorNo != 0 {
			t.Fatalf("unexpected unisearch error_no=%d", resp.ErrorNo)
		}
		for _, file := range resp.Files() {
			if file.FsID != 0 && file.Isdir == 0 && file.Size > 0 && file.Size <= maxIntegrationDownloadSize {
				downloadable = file
				break
			}
		}
	})

	t.Run("Scene/UserInfo", func(t *testing.T) {
		resp, err := sc.UserInfo(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if resp.UK == 0 || userUK != 0 && resp.UK != userUK {
			t.Fatalf("unexpected scene user info uk=%d", resp.UK)
		}
	})

	t.Run("Scene/ListDir", func(t *testing.T) {
		files, err := sc.ListDir(ctx, "/", &ListDirOptions{
			Order:      "time",
			Desc:       true,
			Start:      0,
			Limit:      5,
			FolderOnly: false,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(files) > 5 {
			t.Fatalf("scene list returned %d files, want at most 5", len(files))
		}
	})

	t.Run("Scene/Search", func(t *testing.T) {
		results, err := sc.Search(ctx, &SearchParams{
			Query:    "pdf",
			Dir:      "/",
			Category: []int{4},
			Num:      5,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(results) > 5 {
			t.Fatalf("scene search returned %d files, want at most 5", len(results))
		}
	})

	deviceID := os.Getenv("BDPAN_DEVICE_ID")
	t.Run("API/Nas/IoTQueryUInfo", func(t *testing.T) {
		resp, err := client.Nas.IoTQueryUInfo(ctx, &api.IoTQueryUInfoParams{DeviceID: deviceID})
		if err != nil {
			t.Fatal(err)
		}
		if resp.ErrorCode != 0 {
			t.Fatalf("unexpected iotqueryuinfo error_code=%d", resp.ErrorCode)
		}
	})

	t.Run("Scene/NasUserInfo", func(t *testing.T) {
		resp, err := sc.NasUserInfo(ctx, deviceID)
		if err != nil {
			t.Fatal(err)
		}
		if resp.UK == 0 || userUK != 0 && resp.UK != userUK {
			t.Fatalf("unexpected nas user info uk=%d", resp.UK)
		}
	})

	t.Run("API/Download/ExistingFile", func(t *testing.T) {
		if downloadable == nil {
			t.Skip("搜索结果中没有小于等于 10 MiB 的可下载文件")
		}
		verifyAPIDownload(t, ctx, client, downloadable.FsID, nil)
	})

	t.Run("Scene/DownloadFile/ExistingFile", func(t *testing.T) {
		if downloadable == nil {
			t.Skip("搜索结果中没有小于等于 10 MiB 的可下载文件")
		}
		localPath := tempOutputPath(t)
		result, err := sc.DownloadFile(ctx, &DownloadFileParams{FsID: downloadable.FsID, LocalPath: localPath})
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(localPath)
		if err != nil {
			t.Fatal(err)
		}
		if result.Size <= 0 || info.Size() <= 0 || result.Size != info.Size() {
			t.Fatalf("download size mismatch: result=%d file=%d", result.Size, info.Size())
		}
	})

	if os.Getenv("BDPAN_ALLOW_WRITE_TESTS") != "1" {
		t.Run("WriteOperations", func(t *testing.T) {
			t.Skip("BDPAN_ALLOW_WRITE_TESTS=1 未设置，跳过真实写入和清理闭环")
		})
		return
	}

	testRoot := fmt.Sprintf("/apps/baidu_sdk_full_test_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		if _, err := client.FileManager.Delete(cleanupCtx, 0, []string{testRoot}); err != nil {
			t.Errorf("清理远端测试目录 %s 失败: %v", testRoot, err)
		}
	})

	t.Run("API/FileManager/Mkdir", func(t *testing.T) {
		resp, err := client.FileManager.Mkdir(ctx, &api.MkdirParams{Path: testRoot})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || resp.Isdir != 1 || resp.Path != testRoot {
			t.Fatalf("unexpected mkdir response: errno=%d isdir=%d path=%q", resp.Errno, resp.Isdir, resp.Path)
		}
	})

	t.Run("Scene/MkdirIfNotExist", func(t *testing.T) {
		if err := sc.MkdirIfNotExist(ctx, testRoot); err != nil {
			t.Fatal(err)
		}
	})

	directContent := randomContent(t, 4096)
	directPath := path.Join(testRoot, "api-upload.bin")
	var directFsID int64
	t.Run("API/Upload/PrecreateSliceCreate", func(t *testing.T) {
		resp := uploadDirect(t, ctx, client, directPath, directContent)
		directFsID = resp.FsID
		if resp.FsID == 0 || resp.Path != directPath || resp.Size != int64(len(directContent)) {
			t.Fatalf("unexpected create response: fsid=%d path=%q size=%d", resp.FsID, resp.Path, resp.Size)
		}
	})

	t.Run("API/Download/UploadedFile", func(t *testing.T) {
		if directFsID == 0 {
			t.Skip("直接上传未成功")
		}
		verifyAPIDownload(t, ctx, client, directFsID, directContent)
	})

	sceneContent := randomContent(t, 6144)
	localUpload := tempFileWithContent(t, sceneContent)
	scenePath := path.Join(testRoot, "scene-upload.bin")
	var sceneFsID int64
	t.Run("Scene/UploadFile", func(t *testing.T) {
		resp, err := sc.UploadFile(ctx, &UploadFileParams{
			LocalPath:  localUpload,
			RemotePath: scenePath,
		})
		if err != nil {
			t.Fatal(err)
		}
		sceneFsID = resp.FsID
		if resp.FsID == 0 || resp.Path != scenePath || resp.Size != int64(len(sceneContent)) {
			t.Fatalf("unexpected scene upload response: fsid=%d path=%q size=%d", resp.FsID, resp.Path, resp.Size)
		}
	})

	t.Run("Scene/DownloadFile/UploadedFile", func(t *testing.T) {
		if sceneFsID == 0 {
			t.Skip("Scene 上传未成功")
		}
		localPath := tempOutputPath(t)
		resp, err := sc.DownloadFile(ctx, &DownloadFileParams{FsID: sceneFsID, LocalPath: localPath})
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(localPath)
		if err != nil {
			t.Fatal(err)
		}
		if resp.Size != int64(len(sceneContent)) || !bytes.Equal(got, sceneContent) {
			t.Fatalf("scene download content mismatch: result=%d downloaded=%d", resp.Size, len(got))
		}
	})

	nestedDir := path.Join(testRoot, "nested")
	t.Run("API/FileManager/MkdirNested", func(t *testing.T) {
		resp, err := client.FileManager.Mkdir(ctx, &api.MkdirParams{Path: nestedDir})
		if err != nil {
			t.Fatal(err)
		}
		if resp.Errno != 0 || resp.Isdir != 1 {
			t.Fatalf("unexpected nested mkdir response: errno=%d isdir=%d", resp.Errno, resp.Isdir)
		}
	})

	apiCopyPath := path.Join(testRoot, "api-copy.bin")
	apiRenamedPath := path.Join(testRoot, "api-renamed.bin")
	apiMovedPath := path.Join(nestedDir, "api-moved.bin")
	t.Run("API/FileManager/Copy", func(t *testing.T) {
		resp, err := client.FileManager.Copy(ctx, 0, []*api.CopyMoveItem{{
			Path: directPath, Dest: testRoot, Newname: path.Base(apiCopyPath),
		}}, api.Ptr("fail"))
		if err != nil {
			t.Fatal(err)
		}
		assertManageSuccess(t, resp)
	})

	t.Run("API/FileManager/Rename", func(t *testing.T) {
		resp, err := client.FileManager.Rename(ctx, 0, []*api.RenameItem{{
			Path: apiCopyPath, Newname: path.Base(apiRenamedPath),
		}})
		if err != nil {
			t.Fatal(err)
		}
		assertManageSuccess(t, resp)
	})

	t.Run("API/FileManager/Move", func(t *testing.T) {
		resp, err := client.FileManager.Move(ctx, 0, []*api.CopyMoveItem{{
			Path: apiRenamedPath, Dest: nestedDir, Newname: path.Base(apiMovedPath),
		}}, api.Ptr("fail"))
		if err != nil {
			t.Fatal(err)
		}
		assertManageSuccess(t, resp)
	})

	t.Run("API/FileManager/Delete", func(t *testing.T) {
		resp, err := client.FileManager.Delete(ctx, 0, []string{apiMovedPath})
		if err != nil {
			t.Fatal(err)
		}
		assertManageSuccess(t, resp)
	})

	sceneCopyPath := path.Join(testRoot, "scene-copy.bin")
	sceneRenamedPath := path.Join(testRoot, "scene-renamed.bin")
	sceneMovedPath := path.Join(nestedDir, "scene-moved.bin")
	t.Run("Scene/CopyFile", func(t *testing.T) {
		if err := sc.CopyFile(ctx, scenePath, testRoot, path.Base(sceneCopyPath)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Scene/CopyFileWithOndup", func(t *testing.T) {
		if err := sc.CopyFileWithOndup(ctx, scenePath, testRoot, path.Base(sceneCopyPath), "newcopy"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Scene/RenameFile", func(t *testing.T) {
		if err := sc.RenameFile(ctx, sceneCopyPath, path.Base(sceneRenamedPath)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Scene/MoveFile", func(t *testing.T) {
		if err := sc.MoveFile(ctx, sceneRenamedPath, nestedDir, path.Base(sceneMovedPath)); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Scene/DeleteFile", func(t *testing.T) {
		if err := sc.DeleteFile(ctx, []string{sceneMovedPath}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Scene/ListDir/TestRoot", func(t *testing.T) {
		files, err := sc.ListDir(ctx, testRoot, &ListDirOptions{Order: "name", Limit: 100})
		if err != nil {
			t.Fatal(err)
		}
		if len(files) == 0 {
			t.Fatal("test root unexpectedly empty")
		}
	})
}

func uploadDirect(t *testing.T, ctx context.Context, client *api.Client, remotePath string, content []byte) *api.CreateFileResponse {
	t.Helper()
	hash := md5.Sum(content)
	blockMD5 := hex.EncodeToString(hash[:])
	precreate, err := client.Upload.Precreate(ctx, &api.PrecreateParams{
		Path: remotePath, Size: int64(len(content)), BlockList: []string{blockMD5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if precreate.ReturnType == 2 {
		t.Fatal("unique random integration payload unexpectedly used rapid upload")
	}
	if precreate.UploadID == "" || len(precreate.BlockList) == 0 {
		t.Fatalf("unexpected precreate response: uploadid_empty=%v block_count=%d", precreate.UploadID == "", len(precreate.BlockList))
	}

	uploadedMD5s := make([]string, 0, len(precreate.BlockList))
	for _, partSeq := range precreate.BlockList {
		if partSeq != 0 {
			t.Fatalf("unexpected part sequence for single-slice upload: %d", partSeq)
		}
		slice, err := client.Upload.SliceUpload(ctx, &api.SliceUploadParams{
			Path: remotePath, UploadID: precreate.UploadID, PartSeq: partSeq, File: bytes.NewReader(content),
		})
		if err != nil {
			t.Fatal(err)
		}
		if slice.MD5 == "" {
			t.Fatal("slice upload returned empty md5")
		}
		uploadedMD5s = append(uploadedMD5s, slice.MD5)
	}

	created, err := client.Upload.CreateFile(ctx, &api.CreateFileParams{
		Path: remotePath, Size: int64(len(content)), UploadID: precreate.UploadID, BlockList: uploadedMD5s,
	})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func verifyAPIDownload(t *testing.T, ctx context.Context, client *api.Client, fsID int64, expected []byte) {
	t.Helper()
	meta, err := client.Download.Meta(ctx, &api.MetaParams{FsIDs: []int64{fsID}, Extra: api.Ptr(1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.List) != 1 || meta.List[0].Dlink == "" || meta.List[0].Size <= 0 {
		t.Fatalf("unexpected meta response: count=%d", len(meta.List))
	}
	if meta.List[0].Size > maxIntegrationDownloadSize {
		t.Fatalf("refusing to download %d bytes; limit is %d", meta.List[0].Size, maxIntegrationDownloadSize)
	}

	body, contentLength, err := client.Download.Download(ctx, &api.DownloadParams{Dlink: meta.List[0].Dlink})
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, err := io.ReadAll(io.LimitReader(body, maxIntegrationDownloadSize+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) > maxIntegrationDownloadSize {
		t.Fatalf("download exceeded %d bytes", maxIntegrationDownloadSize)
	}
	if int64(len(got)) != meta.List[0].Size {
		t.Fatalf("download size=%d, meta size=%d", len(got), meta.List[0].Size)
	}
	if contentLength > 0 && contentLength != int64(len(got)) {
		t.Fatalf("content length=%d, downloaded=%d", contentLength, len(got))
	}
	if expected != nil && !bytes.Equal(got, expected) {
		t.Fatal("downloaded content does not match uploaded content")
	}
}

func assertManageSuccess(t *testing.T, resp *api.FileManagerResponse) {
	t.Helper()
	if resp == nil || resp.Errno != 0 {
		t.Fatalf("unexpected file manager response: %#v", resp)
	}
	for i, info := range resp.Info {
		if info == nil || info.Errno != 0 {
			t.Fatalf("file manager item %d failed: %#v", i, info)
		}
	}
}

func randomContent(t *testing.T, size int) []byte {
	t.Helper()
	content := make([]byte, size)
	if _, err := rand.Read(content); err != nil {
		t.Fatal(err)
	}
	return content
}

func tempFileWithContent(t *testing.T, content []byte) string {
	t.Helper()
	file, err := os.CreateTemp("", "baidu-sdk-full-upload-*")
	if err != nil {
		t.Fatal(err)
	}
	name := file.Name()
	if _, err := file.Write(content); err != nil {
		file.Close()
		os.Remove(name)
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		os.Remove(name)
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(name) })
	return name
}

func tempOutputPath(t *testing.T) string {
	t.Helper()
	file, err := os.CreateTemp("", "baidu-sdk-full-download-*")
	if err != nil {
		t.Fatal(err)
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(name)
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(name) })
	return name
}
