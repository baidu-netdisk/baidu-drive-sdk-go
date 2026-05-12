package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownload_Meta_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Query().Get("method") != "filemetas" {
			t.Errorf("method param = %q, want filemetas", r.URL.Query().Get("method"))
		}
		if r.URL.Query().Get("dlink") != "1" {
			t.Errorf("dlink param = %q, want 1", r.URL.Query().Get("dlink"))
		}

		// Verify fsids is a JSON array
		fsids := r.URL.Query().Get("fsids")
		var ids []int64
		if err := json.Unmarshal([]byte(fsids), &ids); err != nil {
			t.Errorf("fsids is not valid JSON array: %v", err)
		}
		if len(ids) != 1 || ids[0] != 123456789 {
			t.Errorf("fsids = %v, want [123456789]", ids)
		}

		w.Write([]byte(`{
			"errno": 0,
			"list": [
				{
					"fs_id": 123456789,
					"path": "/apps/test/file.txt",
					"filename": "file.txt",
					"size": 1024,
					"md5": "ab56b4d92b40713acc5af89985d4b786",
					"dlink": "https://d.pcs.baidu.com/file/xxxxx",
					"isdir": 0,
					"category": 6
				}
			]
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Download.Meta(ctx(), &MetaParams{
		FsIDs: []int64{123456789},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
	if len(resp.List) != 1 {
		t.Fatalf("List length = %d, want 1", len(resp.List))
	}
	f := resp.List[0]
	if f.FsID != 123456789 {
		t.Errorf("FsID = %d, want 123456789", f.FsID)
	}
	if f.Path != "/apps/test/file.txt" {
		t.Errorf("Path = %q, want /apps/test/file.txt", f.Path)
	}
	if f.Filename != "file.txt" {
		t.Errorf("Filename = %q, want file.txt", f.Filename)
	}
	if f.Size != 1024 {
		t.Errorf("Size = %d, want 1024", f.Size)
	}
	if f.MD5 != "ab56b4d92b40713acc5af89985d4b786" {
		t.Errorf("MD5 = %q, want ab56b4d92b40713acc5af89985d4b786", f.MD5)
	}
	if f.Dlink == "" {
		t.Error("Dlink should not be empty")
	}
}

func TestDownload_Meta_MultipleFsIDs(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fsids := r.URL.Query().Get("fsids")
		var ids []int64
		if err := json.Unmarshal([]byte(fsids), &ids); err != nil {
			t.Errorf("fsids parse error: %v", err)
		}
		if len(ids) != 3 {
			t.Errorf("fsids length = %d, want 3", len(ids))
		}

		w.Write([]byte(`{
			"errno": 0,
			"list": [
				{"fs_id": 1, "dlink": "https://d.pcs.baidu.com/1"},
				{"fs_id": 2, "dlink": "https://d.pcs.baidu.com/2"},
				{"fs_id": 3, "dlink": "https://d.pcs.baidu.com/3"}
			]
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Download.Meta(ctx(), &MetaParams{
		FsIDs: []int64{1, 2, 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.List) != 3 {
		t.Errorf("List length = %d, want 3", len(resp.List))
	}
}

func TestDownload_Meta_NilParams(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Download.Meta(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestDownload_Meta_EmptyFsIDs(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Download.Meta(ctx(), &MetaParams{})
	if err == nil {
		t.Fatal("expected error for empty fsids")
	}
}

func TestDownload_Meta_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-9,"errmsg":"file not found"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, err := c.Download.Meta(ctx(), &MetaParams{
		FsIDs: []int64{999},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsErrno(err, -9) {
		t.Errorf("expected errno=-9, got: %v", err)
	}
}

func TestDownload_Meta_WithExtra(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extra := r.URL.Query().Get("extra")
		if extra != "1" {
			t.Errorf("extra = %q, want 1", extra)
		}
		w.Write([]byte(`{
			"errno": 0,
			"list": [{"fs_id": 1, "dlink": "https://example.com", "thumbs": {"url1": "https://thumb1"}}]
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Download.Meta(ctx(), &MetaParams{
		FsIDs: []int64{1},
		Extra: Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.List) != 1 {
		t.Fatalf("List length = %d, want 1", len(resp.List))
	}
}

// ---- Download.Download tests ----

func TestDownload_Download_Success(t *testing.T) {
	fileContent := "hello, baidu netdisk!"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		// Verify User-Agent
		ua := r.Header.Get("User-Agent")
		if ua != "pan.baidu.com" {
			t.Errorf("User-Agent = %q, want pan.baidu.com", ua)
		}

		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))
		w.Write([]byte(fileContent))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	body, size, err := c.Download.Download(ctx(), &DownloadParams{
		Dlink: ts.URL + "/file/download",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(data) != fileContent {
		t.Errorf("body = %q, want %q", string(data), fileContent)
	}
	if size != int64(len(fileContent)) {
		t.Errorf("size = %d, want %d", size, len(fileContent))
	}
}

func TestDownload_Download_NilParams(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, _, err := c.Download.Download(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestDownload_Download_EmptyDlink(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, _, err := c.Download.Download(ctx(), &DownloadParams{})
	if err == nil {
		t.Fatal("expected error for empty dlink")
	}
}

func TestDownload_Download_HTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, _, err := c.Download.Download(ctx(), &DownloadParams{
		Dlink: ts.URL + "/file/download",
	})
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should contain 403, got: %v", err)
	}
}

func TestDownload_Download_LargeFile(t *testing.T) {
	// 4MB file
	data := make([]byte, 4*1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		w.Write(data)
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	body, size, err := c.Download.Download(ctx(), &DownloadParams{
		Dlink: ts.URL + "/file/download",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer body.Close()

	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(got) != len(data) {
		t.Errorf("body length = %d, want %d", len(got), len(data))
	}
	if size != int64(len(data)) {
		t.Errorf("content-length = %d, want %d", size, len(data))
	}
}

func TestDownload_Download_TokenInjected(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("access_token")
		if token != "test-token-123" {
			t.Errorf("access_token = %q, want test-token-123", token)
		}
		w.Write([]byte("ok"))
	}))
	defer ts.Close()

	c := NewClient(
		WithBaseURL(ts.URL),
		WithAccessToken("test-token-123"),
	)
	body, _, err := c.Download.Download(ctx(), &DownloadParams{
		Dlink: ts.URL + "/file/download",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	body.Close()
}

