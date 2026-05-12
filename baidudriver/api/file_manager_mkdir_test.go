package api

import (
	"io"
	"net/http"
	"net/url"
	"testing"
)

// =============================================================================
// Mkdir 测试
// =============================================================================

// TestFileManager_Mkdir 测试创建文件夹。
func TestFileManager_Mkdir(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Query().Get("method") != "create" {
			t.Errorf("method param = %q, want create", r.URL.Query().Get("method"))
		}

		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("path") != "/apps/test/mydir" {
			t.Errorf("path = %q, want /apps/test/mydir", vals.Get("path"))
		}
		if vals.Get("isdir") != "1" {
			t.Errorf("isdir = %q, want 1", vals.Get("isdir"))
		}
		if vals.Get("rtype") != "1" {
			t.Errorf("rtype = %q, want 1", vals.Get("rtype"))
		}

		w.Write([]byte(`{
			"errno": 0,
			"fs_id": 178943825547945,
			"category": 6,
			"path": "/apps/test/mydir",
			"ctime": 1670242019,
			"mtime": 1670242019,
			"isdir": 1,
			"status": 0
		}`))
	})
	defer ts.Close()

	resp, err := c.FileManager.Mkdir(ctx(), &MkdirParams{
		Path:  "/apps/test/mydir",
		Rtype: Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
	if resp.FsID != 178943825547945 {
		t.Errorf("FsID = %d, want 178943825547945", resp.FsID)
	}
	if resp.Category != 6 {
		t.Errorf("Category = %d, want 6", resp.Category)
	}
	if resp.Path != "/apps/test/mydir" {
		t.Errorf("Path = %q, want /apps/test/mydir", resp.Path)
	}
	if resp.Ctime != 1670242019 {
		t.Errorf("Ctime = %d, want 1670242019", resp.Ctime)
	}
	if resp.Mtime != 1670242019 {
		t.Errorf("Mtime = %d, want 1670242019", resp.Mtime)
	}
	if resp.Isdir != 1 {
		t.Errorf("Isdir = %d, want 1", resp.Isdir)
	}
}

// TestFileManager_Mkdir_MinimalParams 测试仅必填参数创建文件夹。
func TestFileManager_Mkdir_MinimalParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("path") != "/test/dir" {
			t.Errorf("path = %q, want /test/dir", vals.Get("path"))
		}
		if vals.Get("isdir") != "1" {
			t.Errorf("isdir = %q, want 1", vals.Get("isdir"))
		}
		// 可选参数不应出现
		if vals.Get("rtype") != "" {
			t.Errorf("rtype should be empty, got %q", vals.Get("rtype"))
		}
		if vals.Get("local_ctime") != "" {
			t.Errorf("local_ctime should be empty, got %q", vals.Get("local_ctime"))
		}
		if vals.Get("local_mtime") != "" {
			t.Errorf("local_mtime should be empty, got %q", vals.Get("local_mtime"))
		}
		if vals.Get("mode") != "" {
			t.Errorf("mode should be empty, got %q", vals.Get("mode"))
		}
		w.Write([]byte(`{"errno":0,"fs_id":1,"category":6,"path":"/test/dir","ctime":1000,"mtime":1000,"isdir":1,"status":0}`))
	})
	defer ts.Close()

	resp, err := c.FileManager.Mkdir(ctx(), &MkdirParams{Path: "/test/dir"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
}

// TestFileManager_Mkdir_AllOptionalParams 测试所有可选参数。
func TestFileManager_Mkdir_AllOptionalParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("rtype") != "0" {
			t.Errorf("rtype = %q, want 0", vals.Get("rtype"))
		}
		if vals.Get("local_ctime") != "1596009229" {
			t.Errorf("local_ctime = %q, want 1596009229", vals.Get("local_ctime"))
		}
		if vals.Get("local_mtime") != "1596009230" {
			t.Errorf("local_mtime = %q, want 1596009230", vals.Get("local_mtime"))
		}
		if vals.Get("mode") != "1" {
			t.Errorf("mode = %q, want 1", vals.Get("mode"))
		}
		w.Write([]byte(`{"errno":0,"fs_id":2,"category":6,"path":"/test/dir2","ctime":1000,"mtime":1000,"isdir":1,"status":0}`))
	})
	defer ts.Close()

	ctime := int64(1596009229)
	mtime := int64(1596009230)
	_, err := c.FileManager.Mkdir(ctx(), &MkdirParams{
		Path:       "/test/dir2",
		Rtype:      Ptr(0),
		LocalCtime: &ctime,
		LocalMtime: &mtime,
		Mode:       Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFileManager_Mkdir_NilParams 测试 nil 参数。
func TestFileManager_Mkdir_NilParams(t *testing.T) {
	c := NewClient()
	_, err := c.FileManager.Mkdir(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

// TestFileManager_Mkdir_EmptyPath 测试空路径。
func TestFileManager_Mkdir_EmptyPath(t *testing.T) {
	c := NewClient()
	_, err := c.FileManager.Mkdir(ctx(), &MkdirParams{Path: ""})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

// TestFileManager_Mkdir_APIError 测试创建文件夹 API 返回错误。
func TestFileManager_Mkdir_APIError(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-8,"errmsg":"file already exist"}`))
	})
	defer ts.Close()

	_, err := c.FileManager.Mkdir(ctx(), &MkdirParams{Path: "/existing/dir"})
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !IsErrno(err, -8) {
		t.Errorf("expected errno=-8, got: %v", err)
	}
}
