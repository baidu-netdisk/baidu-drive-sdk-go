package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestUpload_CreateFile_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Query().Get("method") != "create" {
			t.Errorf("method param = %q, want create", r.URL.Query().Get("method"))
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}

		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		if !strings.Contains(bodyStr, "path=%2Fapps%2Ftest%2Ffile.txt") {
			t.Errorf("body should contain path, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "size=1024") {
			t.Errorf("body should contain size=1024, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "isdir=0") {
			t.Errorf("body should contain isdir=0, got: %s", bodyStr)
		}

		// 验证 uploadid
		vals, _ := url.ParseQuery(bodyStr)
		if vals.Get("uploadid") != "test-upload-id" {
			t.Errorf("uploadid = %q, want test-upload-id", vals.Get("uploadid"))
		}

		// 验证 block_list 是 JSON 数组
		var blockList []string
		if err := json.Unmarshal([]byte(vals.Get("block_list")), &blockList); err != nil {
			t.Errorf("block_list is not valid JSON: %v", err)
		}
		if len(blockList) != 1 || blockList[0] != "ab56b4d92b40713acc5af89985d4b786" {
			t.Errorf("block_list = %v, want [ab56b4d92b40713acc5af89985d4b786]", blockList)
		}

		w.Write([]byte(`{
			"errno": 0,
			"fs_id": 123456789,
			"path": "/apps/test/file.txt",
			"server_filename": "file.txt",
			"size": 1024,
			"ctime": 1700000000,
			"mtime": 1700000000,
			"md5": "ab56b4d92b40713acc5af89985d4b786",
			"isdir": 0,
			"category": 6
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Path:      "/apps/test/file.txt",
		Size:      1024,
		UploadID:  "test-upload-id",
		BlockList: []string{"ab56b4d92b40713acc5af89985d4b786"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FsID != 123456789 {
		t.Errorf("FsID = %d, want 123456789", resp.FsID)
	}
	if resp.Path != "/apps/test/file.txt" {
		t.Errorf("Path = %q, want /apps/test/file.txt", resp.Path)
	}
	if resp.ServerFilename != "file.txt" {
		t.Errorf("ServerFilename = %q, want file.txt", resp.ServerFilename)
	}
	if resp.Size != 1024 {
		t.Errorf("Size = %d, want 1024", resp.Size)
	}
	if resp.MD5 != "ab56b4d92b40713acc5af89985d4b786" {
		t.Errorf("MD5 = %q, want ab56b4d92b40713acc5af89985d4b786", resp.MD5)
	}
}

func TestUpload_CreateFile_WithOptionalParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		if !strings.Contains(bodyStr, "rtype=2") {
			t.Errorf("body should contain rtype=2, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "is_revision=1") {
			t.Errorf("body should contain is_revision=1, got: %s", bodyStr)
		}

		w.Write([]byte(`{
			"errno": 0,
			"fs_id": 999,
			"path": "/apps/test/file.txt",
			"size": 2048
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Path:       "/apps/test/file.txt",
		Size:       2048,
		UploadID:   "uid",
		BlockList:  []string{"abc"},
		RType:      Ptr(2),
		IsRevision: Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.FsID != 999 {
		t.Errorf("FsID = %d, want 999", resp.FsID)
	}
}

func TestUpload_CreateFile_MultipleBlocks(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		var blockList []string
		if err := json.Unmarshal([]byte(vals.Get("block_list")), &blockList); err != nil {
			t.Errorf("block_list parse error: %v", err)
		}
		if len(blockList) != 3 {
			t.Errorf("block_list length = %d, want 3", len(blockList))
		}

		w.Write([]byte(`{"errno": 0, "fs_id": 111, "path": "/f", "size": 100}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Path:      "/f",
		Size:      100,
		UploadID:  "uid",
		BlockList: []string{"md5_1", "md5_2", "md5_3"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpload_CreateFile_NilParams(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Upload.CreateFile(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestUpload_CreateFile_EmptyPath(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Size:      1024,
		UploadID:  "uid",
		BlockList: []string{"abc"},
	})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestUpload_CreateFile_EmptyUploadID(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Path:      "/apps/test/file.txt",
		Size:      1024,
		BlockList: []string{"abc"},
	})
	if err == nil {
		t.Fatal("expected error for empty uploadid")
	}
}

func TestUpload_CreateFile_EmptyBlockList(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Path:     "/apps/test/file.txt",
		Size:     1024,
		UploadID: "uid",
	})
	if err == nil {
		t.Fatal("expected error for empty block_list")
	}
}

func TestUpload_CreateFile_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-8,"errmsg":"file already exist"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Path:      "/apps/test/file.txt",
		Size:      1024,
		UploadID:  "uid",
		BlockList: []string{"abc"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsErrno(err, -8) {
		t.Errorf("expected errno=-8, got: %v", err)
	}
}

func TestUpload_CreateFile_IsDir(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "isdir=1") {
			t.Errorf("body should contain isdir=1, got: %s", string(body))
		}
		w.Write([]byte(`{"errno": 0, "fs_id": 222, "path": "/apps/test/dir", "isdir": 1, "size": 0}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Upload.CreateFile(ctx(), &CreateFileParams{
		Path:      "/apps/test/dir",
		Size:      0,
		IsDir:     Ptr(1),
		UploadID:  "uid",
		BlockList: []string{""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Isdir != 1 {
		t.Errorf("Isdir = %d, want 1", resp.Isdir)
	}
}
