package api

import (
	"net/http"
	"testing"
)

// =============================================================================
// File.List 测试
// =============================================================================

func TestFile_List(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		q := r.URL.Query()
		if q.Get("method") != "list" {
			t.Errorf("method param = %q, want list", q.Get("method"))
		}
		if q.Get("dir") != "/test" {
			t.Errorf("dir = %q, want /test", q.Get("dir"))
		}
		if q.Get("limit") != "100" {
			t.Errorf("limit = %q, want 100", q.Get("limit"))
		}
		if q.Get("order") != "name" {
			t.Errorf("order = %q, want name", q.Get("order"))
		}

		w.Write([]byte(`{
			"errno": 0,
			"list": [
				{
					"fs_id": 12345,
					"path": "/test/a.txt",
					"server_filename": "a.txt",
					"size": 1024,
					"server_mtime": 1670242019,
					"server_ctime": 1670242000,
					"local_mtime": 1670242019,
					"local_ctime": 1670242000,
					"isdir": 0,
					"category": 4,
					"md5": "abc123"
				},
				{
					"fs_id": 67890,
					"path": "/test/subdir",
					"server_filename": "subdir",
					"size": 0,
					"server_mtime": 1670242019,
					"server_ctime": 1670242000,
					"local_mtime": 1670242019,
					"local_ctime": 1670242000,
					"isdir": 1,
					"category": 6,
					"md5": ""
				}
			],
			"request_id": 9876543210,
			"guid": 123
		}`))
	})
	defer ts.Close()

	resp, err := c.File.List(ctx(), &ListParams{
		Dir:   "/test",
		Start: Ptr(0),
		Limit: Ptr(100),
		Order: Ptr("name"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
	if len(resp.List) != 2 {
		t.Fatalf("len(List) = %d, want 2", len(resp.List))
	}
	if resp.List[0].ServerFilename != "a.txt" {
		t.Errorf("List[0].ServerFilename = %q, want a.txt", resp.List[0].ServerFilename)
	}
	if resp.List[0].FsID != 12345 {
		t.Errorf("List[0].FsID = %d, want 12345", resp.List[0].FsID)
	}
	if resp.List[0].Size != 1024 {
		t.Errorf("List[0].Size = %d, want 1024", resp.List[0].Size)
	}
	if resp.List[0].MD5 != "abc123" {
		t.Errorf("List[0].MD5 = %q, want abc123", resp.List[0].MD5)
	}
	if resp.List[1].Isdir != 1 {
		t.Errorf("List[1].Isdir = %d, want 1", resp.List[1].Isdir)
	}
	if resp.RequestID.String() != "9876543210" {
		t.Errorf("RequestID = %s, want 9876543210", resp.RequestID.String())
	}
}

func TestFile_List_DefaultDir(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("dir") != "/" {
			t.Errorf("dir = %q, want /", q.Get("dir"))
		}
		w.Write([]byte(`{"errno":0,"list":[],"request_id":1,"guid":0}`))
	})
	defer ts.Close()

	resp, err := c.File.List(ctx(), &ListParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
}

func TestFile_List_AllOptionalParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("dir") != "/docs" {
			t.Errorf("dir = %q, want /docs", q.Get("dir"))
		}
		if q.Get("order") != "time" {
			t.Errorf("order = %q, want time", q.Get("order"))
		}
		if q.Get("desc") != "1" {
			t.Errorf("desc = %q, want 1", q.Get("desc"))
		}
		if q.Get("start") != "10" {
			t.Errorf("start = %q, want 10", q.Get("start"))
		}
		if q.Get("limit") != "50" {
			t.Errorf("limit = %q, want 50", q.Get("limit"))
		}
		if q.Get("web") != "1" {
			t.Errorf("web = %q, want 1", q.Get("web"))
		}
		if q.Get("folder") != "1" {
			t.Errorf("folder = %q, want 1", q.Get("folder"))
		}
		if q.Get("showempty") != "1" {
			t.Errorf("showempty = %q, want 1", q.Get("showempty"))
		}
		w.Write([]byte(`{"errno":0,"list":[],"request_id":1,"guid":0}`))
	})
	defer ts.Close()

	_, err := c.File.List(ctx(), &ListParams{
		Dir:       "/docs",
		Order:     Ptr("time"),
		Desc:      Ptr(1),
		Start:     Ptr(10),
		Limit:     Ptr(50),
		Web:       Ptr(1),
		Folder:    Ptr(1),
		Showempty: Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFile_List_NilParams(t *testing.T) {
	c := NewClient()
	_, err := c.File.List(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestFile_List_APIError(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	})
	defer ts.Close()

	_, err := c.File.List(ctx(), &ListParams{Dir: "/test"})
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}

func TestFile_List_EmptyResult(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0,"list":[],"request_id":0,"guid":0}`))
	})
	defer ts.Close()

	resp, err := c.File.List(ctx(), &ListParams{Dir: "/empty"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.List) != 0 {
		t.Errorf("len(List) = %d, want 0", len(resp.List))
	}
}

func TestFile_List_WithThumbs(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"errno": 0,
			"list": [
				{
					"fs_id": 111,
					"path": "/test/img.jpg",
					"server_filename": "img.jpg",
					"size": 5120,
					"server_mtime": 1670242019,
					"server_ctime": 1670242000,
					"local_mtime": 1670242019,
					"local_ctime": 1670242000,
					"isdir": 0,
					"category": 3,
					"md5": "def456",
					"thumbs": {
						"url1": "https://example.com/thumb1.jpg",
						"url2": "https://example.com/thumb2.jpg"
					}
				}
			],
			"request_id": 222,
			"guid": 333
		}`))
	})
	defer ts.Close()

	resp, err := c.File.List(ctx(), &ListParams{Dir: "/test", Web: Ptr(1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.List) != 1 {
		t.Fatalf("len(List) = %d, want 1", len(resp.List))
	}
	if resp.List[0].Thumbs == nil {
		t.Fatal("Thumbs should not be nil")
	}
	if resp.List[0].Thumbs["url1"] != "https://example.com/thumb1.jpg" {
		t.Errorf("Thumbs[url1] = %q", resp.List[0].Thumbs["url1"])
	}
}

func TestFile_List_LimitAutoStart(t *testing.T) {
	// 服务端要求 start 和 limit 配合使用，只传 limit 不传 start 时 limit 会被忽略。
	// SDK 应自动补上 start=0。
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("limit") != "5" {
			t.Errorf("limit = %q, want 5", q.Get("limit"))
		}
		if q.Get("start") != "0" {
			t.Errorf("start = %q, want 0 (auto-filled)", q.Get("start"))
		}
		w.Write([]byte(`{"errno":0,"list":[],"request_id":1,"guid":0}`))
	})
	defer ts.Close()

	_, err := c.File.List(ctx(), &ListParams{
		Dir:   "/test",
		Limit: Ptr(5),
		// Start 未设置，SDK 应自动补 start=0
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFile_List_WithDirEmpty(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"errno": 0,
			"list": [
				{
					"fs_id": 111,
					"path": "/test/emptydir",
					"server_filename": "emptydir",
					"size": 0,
					"server_mtime": 1670242019,
					"server_ctime": 1670242000,
					"local_mtime": 1670242019,
					"local_ctime": 1670242000,
					"isdir": 1,
					"category": 6,
					"md5": "",
					"dir_empty": 1
				}
			],
			"request_id": 444,
			"guid": 555
		}`))
	})
	defer ts.Close()

	resp, err := c.File.List(ctx(), &ListParams{Dir: "/test", Showempty: Ptr(1)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.List) != 1 {
		t.Fatalf("len(List) = %d, want 1", len(resp.List))
	}
	if resp.List[0].DirEmpty == nil {
		t.Fatal("DirEmpty should not be nil")
	}
	if *resp.List[0].DirEmpty != 1 {
		t.Errorf("DirEmpty = %d, want 1", *resp.List[0].DirEmpty)
	}
}
