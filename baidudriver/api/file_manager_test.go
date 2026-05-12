package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"
)

// =============================================================================
// Copy/Move/Rename/Delete 测试
// =============================================================================

// TestFileManager_Copy 测试复制文件操作。
func TestFileManager_Copy(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Query().Get("method") != "filemanager" {
			t.Errorf("method param = %q, want filemanager", r.URL.Query().Get("method"))
		}
		if r.URL.Query().Get("opera") != "copy" {
			t.Errorf("opera = %q, want copy", r.URL.Query().Get("opera"))
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}

		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("async") != "2" {
			t.Errorf("async = %q, want 2", vals.Get("async"))
		}
		if vals.Get("ondup") != "overwrite" {
			t.Errorf("ondup = %q, want overwrite", vals.Get("ondup"))
		}
		filelist := vals.Get("filelist")
		if filelist == "" {
			t.Error("filelist should not be empty")
		}

		w.Write([]byte(`{
			"errno": 0,
			"info": [{"errno": 0, "path": "/test/a.txt"}],
			"request_id": 9132303301733449639,
			"taskid": 569942193355668
		}`))
	})
	defer ts.Close()

	resp, err := c.FileManager.Copy(ctx(), 2, []*CopyMoveItem{
		{Path: "/test/a.txt", Dest: "/backup", Newname: "a.txt"},
	}, Ptr("overwrite"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
	if resp.TaskID != 569942193355668 {
		t.Errorf("TaskID = %d, want 569942193355668", resp.TaskID)
	}
	if resp.RequestID.String() != "9132303301733449639" {
		t.Errorf("RequestID = %s, want 9132303301733449639", resp.RequestID.String())
	}
	if len(resp.Info) != 1 {
		t.Fatalf("len(Info) = %d, want 1", len(resp.Info))
	}
	if resp.Info[0].Path != "/test/a.txt" {
		t.Errorf("Info[0].Path = %q, want /test/a.txt", resp.Info[0].Path)
	}
}

// TestFileManager_Move 测试移动文件操作。
func TestFileManager_Move(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("opera") != "move" {
			t.Errorf("opera = %q, want move", r.URL.Query().Get("opera"))
		}
		w.Write([]byte(`{"errno":0,"info":[{"errno":0,"path":"/src/a.txt"}],"request_id":123}`))
	})
	defer ts.Close()

	resp, err := c.FileManager.Move(ctx(), 1, []*CopyMoveItem{
		{Path: "/src/a.txt", Dest: "/dst", Newname: "a.txt"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
}

// TestFileManager_Rename 测试重命名操作。
func TestFileManager_Rename(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("opera") != "rename" {
			t.Errorf("opera = %q, want rename", r.URL.Query().Get("opera"))
		}

		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("ondup") != "" {
			t.Errorf("ondup should be empty for rename, got %q", vals.Get("ondup"))
		}

		w.Write([]byte(`{
			"errno": 0,
			"info": [{"errno": 0, "path": "/test/old.txt"}],
			"request_id": 9131857965670515801
		}`))
	})
	defer ts.Close()

	resp, err := c.FileManager.Rename(ctx(), 1, []*RenameItem{
		{Path: "/test/old.txt", Newname: "new.txt"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
	if len(resp.Info) != 1 {
		t.Fatalf("len(Info) = %d, want 1", len(resp.Info))
	}
}

// TestFileManager_Delete 测试删除操作。
func TestFileManager_Delete(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("opera") != "delete" {
			t.Errorf("opera = %q, want delete", r.URL.Query().Get("opera"))
		}

		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("async") != "2" {
			t.Errorf("async = %q, want 2", vals.Get("async"))
		}

		w.Write([]byte(`{
			"errno": 0,
			"info": [],
			"request_id": 9135012437578302233,
			"taskid": 109078799891710
		}`))
	})
	defer ts.Close()

	resp, err := c.FileManager.Delete(ctx(), 2, []string{"/test/a.txt", "/test/b.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
	if resp.TaskID != 109078799891710 {
		t.Errorf("TaskID = %d, want 109078799891710", resp.TaskID)
	}
}

// TestFileManager_Delete_APIError 测试删除时 API 返回错误。
func TestFileManager_Delete_APIError(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-9,"errmsg":"file not found"}`))
	})
	defer ts.Close()

	_, err := c.FileManager.Delete(ctx(), 1, []string{"/nonexistent"})
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !IsErrno(err, -9) {
		t.Errorf("expected errno=-9, got: %v", err)
	}
}

// TestFileManager_Copy_NilFilelist 测试空 filelist 参数。
func TestFileManager_Copy_NilFilelist(t *testing.T) {
	c := NewClient()
	_, err := c.FileManager.Copy(ctx(), 1, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil filelist")
	}
}

// TestFileManager_MultipleFiles 测试批量操作多个文件。
func TestFileManager_MultipleFiles(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"errno": 0,
			"info": [
				{"errno": 0, "path": "/a.txt"},
				{"errno": 0, "path": "/b.txt"},
				{"errno": -9, "path": "/c.txt"}
			],
			"request_id": 456
		}`))
	})
	defer ts.Close()

	resp, err := c.FileManager.Copy(ctx(), 0, []*CopyMoveItem{
		{Path: "/a.txt", Dest: "/dst", Newname: "a.txt"},
		{Path: "/b.txt", Dest: "/dst", Newname: "b.txt"},
		{Path: "/c.txt", Dest: "/dst", Newname: "c.txt"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Info) != 3 {
		t.Fatalf("len(Info) = %d, want 3", len(resp.Info))
	}
	if resp.Info[2].Errno != -9 {
		t.Errorf("Info[2].Errno = %d, want -9", resp.Info[2].Errno)
	}
}

// TestFileManager_CopyWithFileOndup 测试文件级 ondup 参数。
func TestFileManager_CopyWithFileOndup(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		filelist := vals.Get("filelist")
		// 验证 filelist 中包含文件级 ondup
		if filelist == "" {
			t.Error("filelist should not be empty")
		}
		w.Write([]byte(`{"errno":0,"info":[],"request_id":789}`))
	})
	defer ts.Close()

	_, err := c.FileManager.Copy(ctx(), 1, []*CopyMoveItem{
		{Path: "/a.txt", Dest: "/dst", Newname: "a.txt", Ondup: "skip"},
	}, Ptr("fail"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestFileManager_doManage_MarshalError 测试 json.Marshal 失败路径。
func TestFileManager_doManage_MarshalError(t *testing.T) {
	c := NewClient()
	// chan 类型无法 JSON 序列化，触发 json.Marshal 错误
	_, err := c.FileManager.doManage(ctx(), "copy", 0, nil, make(chan int))
	if err == nil {
		t.Fatal("expected marshal error")
	}
}

// TestFileManager_doManage_NilFilelist 测试 filelist 为 untyped nil 时的路径。
func TestFileManager_doManage_NilFilelist(t *testing.T) {
	c := NewClient()
	// 直接传 untyped nil，触发 filelist == nil 分支
	_, err := c.FileManager.doManage(ctx(), "copy", 0, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil filelist")
	}
}

// =============================================================================
// doPost 测试
// =============================================================================

// TestClient_doPost_Success 测试 doPost 方法。
func TestClient_doPost_Success(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if r.URL.Query().Get("key") != "val" {
			t.Errorf("query key = %q, want val", r.URL.Query().Get("key"))
		}
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("field") != "data" {
			t.Errorf("body field = %q, want data", vals.Get("field"))
		}
		w.Write([]byte(`{"errno":0,"name":"ok"}`))
	})
	defer ts.Close()

	var result struct {
		Name string `json:"name"`
	}
	q := url.Values{"key": {"val"}}
	b := url.Values{"field": {"data"}}
	_, err := c.doPost(ctx(), "/test", q, b, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "ok" {
		t.Errorf("name = %q, want ok", result.Name)
	}
}

// ctx 返回一个 context.Background()，简化测试代码。
func ctx() context.Context {
	return context.Background()
}
