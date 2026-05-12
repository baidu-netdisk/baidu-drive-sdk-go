package scene

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func TestDownloadFile_Success(t *testing.T) {
	fileContent := "hello, download test!"

	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 123,
					"path": "/apps/test/file.txt",
					"filename": "file.txt",
					"size": %d,
					"md5": "test-md5",
					"dlink": "%s/download?param=1"
				}]
			}`, len(fileContent), serverURL)))
			return
		}

		// Download endpoint
		if r.URL.Path == "/download" {
			ua := r.Header.Get("User-Agent")
			if ua != "pan.baidu.com" {
				t.Errorf("User-Agent = %q, want pan.baidu.com", ua)
			}
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))
			w.Write([]byte(fileContent))
			return
		}

		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()
	serverURL = ts.URL

	outFile, err := os.CreateTemp("", "download_test_*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	result, err := sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      123,
		LocalPath: outFile.Name(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Size != int64(len(fileContent)) {
		t.Errorf("Size = %d, want %d", result.Size, len(fileContent))
	}
	if result.Path != "/apps/test/file.txt" {
		t.Errorf("Path = %q, want /apps/test/file.txt", result.Path)
	}

	data, err := os.ReadFile(outFile.Name())
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("file content = %q, want %q", string(data), fileContent)
	}
}

func TestDownloadFile_NilParams(t *testing.T) {
	c := api.NewClient(api.WithBaseURL("https://example.com"))
	sc := New(c)
	_, err := sc.DownloadFile(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestDownloadFile_EmptyLocalPath(t *testing.T) {
	c := api.NewClient(api.WithBaseURL("https://example.com"))
	sc := New(c)
	_, err := sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID: 123,
	})
	if err == nil {
		t.Fatal("expected error for empty local path")
	}
}

func TestDownloadFile_ZeroFsID(t *testing.T) {
	c := api.NewClient(api.WithBaseURL("https://example.com"))
	sc := New(c)
	_, err := sc.DownloadFile(context.Background(), &DownloadFileParams{
		LocalPath: "/tmp/test",
	})
	if err == nil {
		t.Fatal("expected error for zero fsid")
	}
}

func TestDownloadFile_MetaRetry(t *testing.T) {
	fileContent := "retry download"
	metaAttempts := 0

	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			metaAttempts++
			if metaAttempts < 2 {
				w.Write([]byte(`{"errno": -1, "errmsg": "server busy"}`))
				return
			}
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 456,
					"path": "/apps/test/retry.txt",
					"filename": "retry.txt",
					"size": %d,
					"md5": "test-md5",
					"dlink": "%s/download"
				}]
			}`, len(fileContent), serverURL)))
			return
		}
		w.Write([]byte(fileContent))
	}))
	defer ts.Close()
	serverURL = ts.URL

	outFile, err := os.CreateTemp("", "download_retry_test_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	_, err = sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      456,
		LocalPath: outFile.Name(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metaAttempts < 2 {
		t.Errorf("meta attempts = %d, want >= 2", metaAttempts)
	}
}

func TestDownloadFile_DownloadRetry(t *testing.T) {
	fileContent := "download retry content"
	downloadAttempts := 0

	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 789,
					"path": "/apps/test/dlretry.txt",
					"filename": "dlretry.txt",
					"size": %d,
					"md5": "test-md5",
					"dlink": "%s/download"
				}]
			}`, len(fileContent), serverURL)))
			return
		}

		downloadAttempts++
		if downloadAttempts < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("server busy"))
			return
		}
		w.Write([]byte(fileContent))
	}))
	defer ts.Close()
	serverURL = ts.URL

	outFile, err := os.CreateTemp("", "download_dl_retry_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	_, err = sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      789,
		LocalPath: outFile.Name(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if downloadAttempts < 2 {
		t.Errorf("download attempts = %d, want >= 2", downloadAttempts)
	}

	data, err := os.ReadFile(outFile.Name())
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("content = %q, want %q", string(data), fileContent)
	}
}

func TestDownloadFile_ToWriter(t *testing.T) {
	fileContent := "writer download test"

	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 100,
					"path": "/apps/test/writer.txt",
					"filename": "writer.txt",
					"size": %d,
					"md5": "test-md5",
					"dlink": "%s/download"
				}]
			}`, len(fileContent), serverURL)))
			return
		}
		w.Write([]byte(fileContent))
	}))
	defer ts.Close()
	serverURL = ts.URL

	outFile, err := os.CreateTemp("", "download_writer_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	_, err = sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      100,
		LocalPath: outFile.Name(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(outFile.Name())
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("content = %q, want %q", string(data), fileContent)
	}
}

func TestDownloadFile_MetaAllFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(`{"errno":-1,"errmsg":"server busy"}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()

	outFile, err := os.CreateTemp("", "download_metafail_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = sc.DownloadFile(ctx, &DownloadFileParams{
		FsID:      111,
		LocalPath: outFile.Name(),
	})
	if err == nil {
		t.Fatal("expected error when meta always fails")
	}
	if !strings.Contains(err.Error(), "download meta") {
		t.Errorf("error = %q, want to contain 'download meta'", err.Error())
	}
}

func TestDownloadFile_DownloadAllFail(t *testing.T) {
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 222,
					"path": "/apps/test/dlfail.txt",
					"filename": "dlfail.txt",
					"size": 100,
					"md5": "test-md5",
					"dlink": "%s/download"
				}]
			}`, serverURL)))
			return
		}
		if r.URL.Path == "/download" {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("server busy"))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()
	serverURL = ts.URL

	outFile, err := os.CreateTemp("", "download_dlfail_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err = sc.DownloadFile(ctx, &DownloadFileParams{
		FsID:      222,
		LocalPath: outFile.Name(),
	})
	if err == nil {
		t.Fatal("expected error when download always fails")
	}
	if !strings.Contains(err.Error(), "download file") {
		t.Errorf("error = %q, want to contain 'download file'", err.Error())
	}
}

func TestDownloadFile_WriteCopyError(t *testing.T) {
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 333,
					"path": "/apps/test/copyerr.txt",
					"filename": "copyerr.txt",
					"size": 10000,
					"md5": "test-md5",
					"dlink": "%s/download"
				}]
			}`, serverURL)))
			return
		}
		if r.URL.Path == "/download" {
			// Claim a large Content-Length but close connection early
			w.Header().Set("Content-Length", "10000")
			w.Write([]byte("partial"))
			// Hijack to force-close the connection
			if hj, ok := w.(http.Hijacker); ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
			}
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()
	serverURL = ts.URL

	outFile, err := os.CreateTemp("", "download_copyerr_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	_, err = sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      333,
		LocalPath: outFile.Name(),
	})
	if err == nil {
		t.Fatal("expected error for io.Copy failure")
	}
	if !strings.Contains(err.Error(), "write file") {
		t.Errorf("error = %q, want to contain 'write file'", err.Error())
	}
}

func TestDownloadFile_MetaEmptyList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(`{"errno":0,"list":[]}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()

	outFile, err := os.CreateTemp("", "download_emptylist_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	_, err = sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      999,
		LocalPath: outFile.Name(),
	})
	if err == nil {
		t.Fatal("expected error for empty meta list")
	}
	if !strings.Contains(err.Error(), "empty list") {
		t.Errorf("error = %q, want to contain 'empty list'", err.Error())
	}
}

func TestDownloadFile_EmptyDlink(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(`{
				"errno": 0,
				"list": [{
					"fs_id": 888,
					"path": "/apps/test/nodlink.txt",
					"filename": "nodlink.txt",
					"size": 100,
					"md5": "test-md5",
					"dlink": ""
				}]
			}`))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()

	outFile, err := os.CreateTemp("", "download_nodlink_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	_, err = sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      888,
		LocalPath: outFile.Name(),
	})
	if err == nil {
		t.Fatal("expected error for empty dlink")
	}
	if !strings.Contains(err.Error(), "empty dlink") {
		t.Errorf("error = %q, want to contain 'empty dlink'", err.Error())
	}
}

func TestDownloadFile_CreateFileError(t *testing.T) {
	fileContent := "create failure test"

	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 777,
					"path": "/apps/test/fail.txt",
					"filename": "fail.txt",
					"size": %d,
					"md5": "test-md5",
					"dlink": "%s/download"
				}]
			}`, len(fileContent), serverURL)))
			return
		}
		if r.URL.Path == "/download" {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))
			w.Write([]byte(fileContent))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()
	serverURL = ts.URL

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	// Use an invalid path so os.Create fails
	_, err := sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      777,
		LocalPath: "/nonexistent-dir-abc123/impossible/file.txt",
	})
	if err == nil {
		t.Fatal("expected error for os.Create failure")
	}
	if !strings.Contains(err.Error(), "create output file") {
		t.Errorf("error = %q, want to contain 'create output file'", err.Error())
	}
}

func TestDownloadFile_NoContentLength(t *testing.T) {
	fileContent := "no content length"

	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		if method == "filemetas" {
			w.Write([]byte(fmt.Sprintf(`{
				"errno": 0,
				"list": [{
					"fs_id": 666,
					"path": "/apps/test/nolen.txt",
					"filename": "nolen.txt",
					"size": %d,
					"md5": "test-md5",
					"dlink": "%s/download"
				}]
			}`, len(fileContent), serverURL)))
			return
		}
		if r.URL.Path == "/download" {
			// Do NOT set Content-Length header; use chunked transfer
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Write([]byte(fileContent))
			return
		}
		t.Errorf("unexpected request: %s %s", r.Method, r.URL)
	}))
	defer ts.Close()
	serverURL = ts.URL

	outFile, err := os.CreateTemp("", "download_nolen_*")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	outFile.Close()
	defer os.Remove(outFile.Name())

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := New(c)

	result, err := sc.DownloadFile(context.Background(), &DownloadFileParams{
		FsID:      666,
		LocalPath: outFile.Name(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When contentLength <= 0, size should fall back to written bytes
	if result.Size != int64(len(fileContent)) {
		t.Errorf("Size = %d, want %d (fallback to written bytes)", result.Size, len(fileContent))
	}

	data, err := os.ReadFile(outFile.Name())
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("content = %q, want %q", string(data), fileContent)
	}
}
