package scene

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func TestUploadFile_Success_SmallFile(t *testing.T) {
	content := []byte("hello, upload test!")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	h := md5.Sum(content)
	contentMD5 := hex.EncodeToString(h[:])

	callOrder := []string{}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			callOrder = append(callOrder, "precreate")
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"uploadid": "test-upload-id",
				"block_list": [0],
				"return_type": 1
			}`)))
		case "upload":
			callOrder = append(callOrder, "upload")
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"md5": "%s"
			}`, contentMD5)))
		case "create":
			callOrder = append(callOrder, "create")
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"fs_id": 12345,
				"path": "/apps/test/file.txt",
				"size": %d,
				"md5": "%s"
			}`, len(content), contentMD5)))
		default:
			t.Errorf("unexpected method: %s", method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	result, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/file.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FsID != 12345 {
		t.Errorf("FsID = %d, want 12345", result.FsID)
	}
	if result.Path != "/apps/test/file.txt" {
		t.Errorf("Path = %q, want /apps/test/file.txt", result.Path)
	}
	if result.MD5 != contentMD5 {
		t.Errorf("MD5 = %q, want %q", result.MD5, contentMD5)
	}

	// Verify call order: precreate → upload → create
	expected := []string{"precreate", "upload", "create"}
	if len(callOrder) != len(expected) {
		t.Fatalf("call order length = %d, want %d: %v", len(callOrder), len(expected), callOrder)
	}
	for i, v := range expected {
		if callOrder[i] != v {
			t.Errorf("callOrder[%d] = %q, want %q", i, callOrder[i], v)
		}
	}
}

func TestUploadFile_RapidUpload(t *testing.T) {
	content := []byte("rapid upload content")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			// return_type=2 表示秒传成功
			w.Write([]byte(`{
				"errno": 0,
				"uploadid": "",
				"block_list": [],
				"return_type": 2
			}`))
		default:
			t.Errorf("unexpected method after rapid upload: %s", method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	result, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/rapid.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil for rapid upload")
	}
}

func TestUploadFile_MultipleSlices(t *testing.T) {
	// Create a file larger than DefaultSliceSize for chunking
	sliceSize := 1024 // Use small slice for testing
	content := bytes.Repeat([]byte("a"), sliceSize*3+100)
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	uploadedSlices := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			w.Write([]byte(`{
				"errno": 0,
				"uploadid": "multi-slice-uid",
				"block_list": [0, 1, 2, 3],
				"return_type": 1
			}`))
		case "upload":
			uploadedSlices++
			w.Write([]byte(fmt.Sprintf(`{"errno": 0, "md5": "slice_md5_%d"}`, uploadedSlices)))
		case "create":
			// Verify block_list in body
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)
			var vals = make(map[string][]string)
			for _, kv := range strings.Split(bodyStr, "&") {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) == 2 {
					vals[parts[0]] = append(vals[parts[0]], parts[1])
				}
			}
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"fs_id": 99999,
				"path": "/apps/test/big.bin",
				"size": %d
			}`, len(content))))
		default:
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	result, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/big.bin",
		SliceSize:  int64(sliceSize),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uploadedSlices != 4 {
		t.Errorf("uploaded slices = %d, want 4", uploadedSlices)
	}
	if result.FsID != 99999 {
		t.Errorf("FsID = %d, want 99999", result.FsID)
	}
}

func TestUploadFile_NilParams(t *testing.T) {
	c := api.NewClient(api.WithBaseURL("https://example.com"))
	sc := New(c)
	_, err := sc.UploadFile(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestUploadFile_EmptyLocalPath(t *testing.T) {
	c := api.NewClient(api.WithBaseURL("https://example.com"))
	sc := New(c)
	_, err := sc.UploadFile(context.Background(), &UploadFileParams{
		RemotePath: "/apps/test/file.txt",
	})
	if err == nil {
		t.Fatal("expected error for empty local path")
	}
}

func TestUploadFile_EmptyRemotePath(t *testing.T) {
	c := api.NewClient(api.WithBaseURL("https://example.com"))
	sc := New(c)
	_, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath: "/tmp/nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for empty remote path")
	}
}

func TestUploadFile_FileNotFound(t *testing.T) {
	c := api.NewClient(api.WithBaseURL("https://example.com"))
	sc := New(c)
	_, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath:  "/tmp/definitely-not-exists-12345",
		RemotePath: "/apps/test/file.txt",
	})
	if err == nil {
		t.Fatal("expected error for file not found")
	}
}

func TestUploadFile_SliceRetry(t *testing.T) {
	content := []byte("retry content")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	h := md5.Sum(content)
	contentMD5 := hex.EncodeToString(h[:])

	sliceAttempts := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			w.Write([]byte(`{
				"errno": 0,
				"uploadid": "retry-uid",
				"block_list": [0],
				"return_type": 1
			}`))
		case "upload":
			sliceAttempts++
			if sliceAttempts < 2 {
				// First attempt fails
				w.Write([]byte(`{"errno": -1, "errmsg": "server busy"}`))
				return
			}
			w.Write([]byte(fmt.Sprintf(`{"errno": 0, "md5": "%s"}`, contentMD5)))
		case "create":
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"fs_id": 111,
				"path": "/apps/test/retry.txt",
				"md5": "%s"
			}`, contentMD5)))
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	result, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/retry.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FsID != 111 {
		t.Errorf("FsID = %d, want 111", result.FsID)
	}
	if sliceAttempts < 2 {
		t.Errorf("slice attempts = %d, want >= 2 (should have retried)", sliceAttempts)
	}
}

func TestUploadFile_WithRType(t *testing.T) {
	content := []byte("rtype content")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	h := md5.Sum(content)
	contentMD5 := hex.EncodeToString(h[:])

	var precreateRType string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			body, _ := io.ReadAll(r.Body)
			for _, kv := range strings.Split(string(body), "&") {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) == 2 && parts[0] == "rtype" {
					precreateRType = parts[1]
				}
			}
			w.Write([]byte(`{
				"errno": 0,
				"uploadid": "rtype-uid",
				"block_list": [0],
				"return_type": 1
			}`))
		case "upload":
			w.Write([]byte(fmt.Sprintf(`{"errno": 0, "md5": "%s"}`, contentMD5)))
		case "create":
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"fs_id": 222,
				"path": "/apps/test/rtype.txt",
				"md5": "%s"
			}`, contentMD5)))
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	_, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/rtype.txt",
		RType:      api.Ptr(2),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if precreateRType != "2" {
		t.Errorf("precreate rtype = %q, want 2", precreateRType)
	}
}

// computeBlockMD5List 和 UploadFile 中使用的 MD5 计算辅助验证
func TestUploadFile_BlockListMD5(t *testing.T) {
	content := []byte("block list md5 test content")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	h := md5.Sum(content)
	contentMD5 := hex.EncodeToString(h[:])

	var receivedBlockList string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			body, _ := io.ReadAll(r.Body)
			for _, kv := range strings.Split(string(body), "&") {
				parts := strings.SplitN(kv, "=", 2)
				if len(parts) == 2 && parts[0] == "block_list" {
					decoded, _ := decodeURLComponent(parts[1])
					receivedBlockList = decoded
				}
			}
			w.Write([]byte(`{
				"errno": 0,
				"uploadid": "md5-uid",
				"block_list": [0],
				"return_type": 1
			}`))
		case "upload":
			w.Write([]byte(fmt.Sprintf(`{"errno": 0, "md5": "%s"}`, contentMD5)))
		case "create":
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"fs_id": 333,
				"path": "/apps/test/md5.txt",
				"md5": "%s"
			}`, contentMD5)))
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	_, err := sc.UploadFile(context.Background(), &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/md5.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify block_list is a JSON array of MD5 hashes
	if receivedBlockList == "" {
		t.Fatal("block_list should not be empty")
	}
	var blockList []string
	if err := json.Unmarshal([]byte(receivedBlockList), &blockList); err != nil {
		t.Fatalf("block_list is not valid JSON: %v", err)
	}
	if len(blockList) != 1 {
		t.Fatalf("block_list length = %d, want 1", len(blockList))
	}
	if blockList[0] != contentMD5 {
		t.Errorf("block_list[0] = %q, want %q", blockList[0], contentMD5)
	}
}

func TestUploadFile_PrecreateAllFail(t *testing.T) {
	content := []byte("precreate fail content")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			// Always return error — all retries should fail
			w.Write([]byte(`{"errno": -1, "errmsg": "server busy"}`))
		default:
			t.Errorf("unexpected method: %s (should not reach upload/create)", method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := sc.UploadFile(ctx, &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/precreate_fail.txt",
	})
	if err == nil {
		t.Fatal("expected error when precreate always fails")
	}
	if !strings.Contains(err.Error(), "precreate") {
		t.Errorf("error = %q, want to contain 'precreate'", err.Error())
	}
}

func TestUploadFile_CreateFileError(t *testing.T) {
	content := []byte("create fail content")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	h := md5.Sum(content)
	contentMD5 := hex.EncodeToString(h[:])

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			w.Write([]byte(`{
				"errno": 0,
				"uploadid": "create-fail-uid",
				"block_list": [0],
				"return_type": 1
			}`))
		case "upload":
			w.Write([]byte(fmt.Sprintf(`{"errno": 0, "md5": "%s"}`, contentMD5)))
		case "create":
			// Always return error
			w.Write([]byte(`{"errno": -1, "errmsg": "create failed"}`))
		default:
			t.Errorf("unexpected method: %s", method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := sc.UploadFile(ctx, &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/create_fail.txt",
	})
	if err == nil {
		t.Fatal("expected error when create always fails")
	}
	if !strings.Contains(err.Error(), "create file") {
		t.Errorf("error = %q, want to contain 'create file'", err.Error())
	}
}

func TestUploadFile_SliceUploadAllFail(t *testing.T) {
	content := []byte("slice fail content")
	tmpFile := writeTempFile(t, content)
	defer os.Remove(tmpFile)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			w.Write([]byte(`{
				"errno": 0,
				"uploadid": "slice-fail-uid",
				"block_list": [0],
				"return_type": 1
			}`))
		case "upload":
			// Always fail
			w.Write([]byte(`{"errno": -1, "errmsg": "upload failed"}`))
		default:
			t.Errorf("unexpected method: %s", method)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := New(c)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := sc.UploadFile(ctx, &UploadFileParams{
		LocalPath:  tmpFile,
		RemotePath: "/apps/test/slice_fail.txt",
	})
	if err == nil {
		t.Fatal("expected error when slice upload always fails")
	}
	if !strings.Contains(err.Error(), "slice upload") {
		t.Errorf("error = %q, want to contain 'slice upload'", err.Error())
	}
}

// ---- helpers ----

func writeTempFile(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "upload_test_*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

func decodeURLComponent(s string) (string, error) {
	// Simple URL decode for %XX
	result := strings.ReplaceAll(s, "%5B", "[")
	result = strings.ReplaceAll(result, "%5D", "]")
	result = strings.ReplaceAll(result, "%22", "\"")
	result = strings.ReplaceAll(result, "%2C", ",")
	result = strings.ReplaceAll(result, "+", " ")
	return result, nil
}
