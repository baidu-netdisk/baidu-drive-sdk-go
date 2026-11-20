package api

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
)

const fileInfoJSON = `{
  "errno": 0,
  "errmsg": "succ",
  "request_id": "42",
  "guid": 7,
  "info": [{
    "fs_id": 123,
    "path": "/docs/readme.pdf",
    "server_filename": "readme.pdf",
    "filename": "readme.pdf",
    "size": 1024,
    "server_mtime": 11,
    "server_ctime": 10,
    "local_mtime": 9,
    "local_ctime": 8,
    "isdir": 0,
    "category": 4,
    "md5": "abc",
    "privacy": 1,
    "unlist": 0,
    "server_atime": 12,
    "dir_empty": 1,
    "empty": 0,
    "thumbs": {"url1": "https://example.test/thumb"},
    "dlink": "https://example.test/file",
    "height": 720,
    "width": 1280,
    "date_taken": 13,
    "orientation": "0",
    "media_info": {"duration": 4},
    "object_key": "object",
    "oper_id": 14,
    "share": 1,
    "extent_tinyint1": 2
  }]
}`

func TestFileInfo_ListMethods(t *testing.T) {
	methods := []struct {
		name string
		call func(context.Context, *Client) (*FileListInfoResponse, error)
	}{
		{"doclist", func(ctx context.Context, c *Client) (*FileListInfoResponse, error) {
			p := &DocumentListParams{ParentPath: "/docs", Page: Ptr(2), Num: Ptr(10), Order: Ptr("time"), Desc: Ptr(1), Recursion: Ptr(1), Web: Ptr(1)}
			return c.File.DocumentList(ctx, p)
		}},
		{"imagelist", func(ctx context.Context, c *Client) (*FileListInfoResponse, error) {
			p := &ImageListParams{ParentPath: "/docs", Page: Ptr(2), Num: Ptr(10), Order: Ptr("time"), Desc: Ptr(1), Recursion: Ptr(1), Web: Ptr(1)}
			return c.File.ImageList(ctx, p)
		}},
		{"videolist", func(ctx context.Context, c *Client) (*FileListInfoResponse, error) {
			p := &VideoListParams{ParentPath: "/docs", Page: Ptr(2), Num: Ptr(10), Order: Ptr("time"), Desc: Ptr(1), Recursion: Ptr(1), Web: Ptr(1)}
			return c.File.VideoList(ctx, p)
		}},
		{"btlist", func(ctx context.Context, c *Client) (*FileListInfoResponse, error) {
			p := &BTListParams{ParentPath: "/docs", Page: Ptr(2), Num: Ptr(10), Order: Ptr("time"), Desc: Ptr(1), Recursion: Ptr(1), Web: Ptr(1)}
			return c.File.BTList(ctx, p)
		}},
	}

	for _, tc := range methods {
		t.Run(tc.name, func(t *testing.T) {
			ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != "/rest/2.0/xpan/file" {
					t.Fatalf("request = %s %s, want GET /rest/2.0/xpan/file", r.Method, r.URL.Path)
				}
				q := r.URL.Query()
				if q.Get("method") != tc.name || q.Get("parent_path") != "/docs" || q.Get("page") != "2" || q.Get("num") != "10" || q.Get("order") != "time" || q.Get("desc") != "1" || q.Get("recursion") != "1" || q.Get("web") != "1" {
					t.Errorf("query = %v", q)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(fileInfoJSON))
			})
			defer ts.Close()

			resp, err := tc.call(context.Background(), c)
			if err != nil {
				t.Fatal(err)
			}
			assertFileInfo(t, resp.Info[0])
			if resp.RequestID.String() != "42" || resp.Guid != 7 {
				t.Fatalf("response metadata = request_id %s guid %d", resp.RequestID, resp.Guid)
			}
		})
	}

	// DocList/BtList are compatibility aliases and must use the same wire method.
	t.Run("aliases", func(t *testing.T) {
		var seen []string
		ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
			seen = append(seen, r.URL.Query().Get("method"))
			_, _ = w.Write([]byte(`{"errno":0,"info":[]}`))
		})
		defer ts.Close()
		if _, err := c.File.DocList(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		if _, err := c.File.BtList(context.Background(), nil); err != nil {
			t.Fatal(err)
		}
		if len(seen) != 2 || seen[0] != "doclist" || seen[1] != "btlist" {
			t.Fatalf("methods = %v", seen)
		}
	})
}

func TestFileInfo_ListAll(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/2.0/xpan/multimedia" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q := r.URL.Query()
		want := map[string]string{"method": "listall", "path": "/根目录", "recursion": "1", "order": "size", "desc": "1", "start": "3", "limit": "20", "ctime": "10", "mtime": "11", "web": "1", "device_id": "device"}
		for key, value := range want {
			if q.Get(key) != value {
				t.Errorf("query %s = %q, want %q", key, q.Get(key), value)
			}
		}
		_, _ = w.Write([]byte(`{"errno":0,"has_more":1,"cursor":20,"list":[]}`))
	})
	defer ts.Close()
	resp, err := c.File.ListAll(context.Background(), &ListAllParams{
		Path: "/根目录", Recursion: Ptr(1), Order: Ptr("size"), Desc: Ptr(1), Start: Ptr(3), Limit: Ptr(20), Ctime: Ptr(int64(10)), Mtime: Ptr(int64(11)), Web: Ptr(1), DeviceID: "device",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.HasMore != 1 || resp.Cursor != 20 {
		t.Fatalf("pagination = %+v", resp)
	}
}

func TestFileInfo_CategoryCountAndList(t *testing.T) {
	t.Run("count", func(t *testing.T) {
		ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/categoryinfo" {
				t.Errorf("path = %q", r.URL.Path)
			}
			q := r.URL.Query()
			if q.Get("category") != "4" || q.Get("parent_path") != "/docs" || q.Get("recursion") != "1" {
				t.Errorf("query = %v", q)
			}
			_, _ = w.Write([]byte(`{"errno":0,"info":{"4":{"total":3,"size":99,"count":3,"real_server_mtime_2":123}},"request_id":9}`))
		})
		defer ts.Close()
		resp, err := c.File.CategoryCount(context.Background(), &CategoryCountParams{Category: 4, ParentPath: "/docs", Recursion: Ptr(1)})
		if err != nil || resp.Info["4"].Count != 3 || resp.Info["4"].RealServerMtime2.String() != "123" {
			t.Fatalf("response=%+v err=%v", resp, err)
		}
	})

	t.Run("list", func(t *testing.T) {
		ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/rest/2.0/xpan/multimedia" {
				t.Errorf("path = %q", r.URL.Path)
			}
			q := r.URL.Query()
			want := map[string]string{"method": "categorylist", "category": "3,4", "show_dir": "0", "parent_path": "/docs", "recursion": "0", "ext": "jpg,png", "start": "2", "limit": "50", "order": "name", "desc": "1", "device_id": "device"}
			for key, value := range want {
				if q.Get(key) != value {
					t.Errorf("query %s = %q, want %q", key, q.Get(key), value)
				}
			}
			_, _ = w.Write([]byte(`{"errno":0,"cursor":52,"has_more":0,"list":[]}`))
		})
		defer ts.Close()
		resp, err := c.File.CategoryList(context.Background(), &CategoryListParams{Category: "3,4", ShowDir: Ptr(0), ParentPath: "/docs", Recursion: Ptr(0), Ext: "jpg,png", Start: Ptr(2), Limit: Ptr(50), Order: Ptr("name"), Desc: Ptr(1), DeviceID: "device"})
		if err != nil || resp.Cursor != 52 {
			t.Fatalf("response=%+v err=%v", resp, err)
		}
	})
}

func TestFileInfo_QueryAndKeywordSearch(t *testing.T) {
	t.Run("query", func(t *testing.T) {
		ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if r.URL.Path != "/rest/2.0/xpan/multimedia" || q.Get("method") != "filemetas" || q.Get("fsids") != "[1,2]" || q.Get("dlink") != "1" || q.Get("path") != "/shared" || q.Get("thumb") != "1" || q.Get("extra") != "1" || q.Get("needmedia") != "1" || q.Get("detail") != "1" || q.Get("device_id") != "device" || q.Get("from_apaas") != "1" {
				t.Errorf("request = %s %v", r.URL.Path, q)
			}
			_, _ = w.Write([]byte(`{"errno":0,"names":{"uk":123},"list":[{"fs_id":1,"dlink":"https://example.test/file"}]}`))
		})
		defer ts.Close()
		resp, err := c.File.QueryFileInfo(context.Background(), &QueryFileInfoParams{FsIDs: []int64{1, 2}, Dlink: Ptr(1), Path: "/shared", Thumb: Ptr(1), Extra: Ptr(1), NeedMedia: Ptr(1), Detail: Ptr(1), DeviceID: "device", FromAPaas: Ptr(1)})
		if err != nil || len(resp.List) != 1 || resp.List[0].Dlink == "" {
			t.Fatalf("response=%+v err=%v", resp, err)
		}
	})

	t.Run("keyword", func(t *testing.T) {
		ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			want := map[string]string{"method": "search", "key": "报告", "dir": "/docs", "category": "4", "recursion": "1", "web": "1", "device_id": "device", "num": "500"}
			for key, value := range want {
				if q.Get(key) != value {
					t.Errorf("query %s = %q, want %q", key, q.Get(key), value)
				}
			}
			_, _ = w.Write([]byte(`{"errno":0,"has_more":0,"list":[]}`))
		})
		defer ts.Close()
		resp, err := c.File.KeywordSearch(context.Background(), &KeywordSearchParams{Key: "报告", Dir: "/docs", Category: Ptr(4), Recursion: Ptr(1), Web: Ptr(1), DeviceID: "device"})
		if err != nil || resp.HasMore != 0 {
			t.Fatalf("response=%+v err=%v", resp, err)
		}
	})
}

func TestFileInfo_Validation(t *testing.T) {
	var requests atomic.Int32
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`{"errno":0}`))
	})
	defer ts.Close()

	cases := []struct {
		name string
		call func() error
	}{
		{"listall nil", func() error { _, err := c.File.ListAll(context.Background(), nil); return err }},
		{"listall relative path", func() error { _, err := c.File.ListAll(context.Background(), &ListAllParams{Path: "docs"}); return err }},
		{"listall bad order", func() error {
			_, err := c.File.ListAll(context.Background(), &ListAllParams{Path: "/docs", Order: Ptr("bad")})
			return err
		}},
		{"listall bad limit", func() error {
			_, err := c.File.ListAll(context.Background(), &ListAllParams{Path: "/docs", Limit: Ptr(1001)})
			return err
		}},
		{"category count bad category", func() error {
			_, err := c.File.CategoryCount(context.Background(), &CategoryCountParams{Category: 0})
			return err
		}},
		{"category list empty", func() error { _, err := c.File.CategoryList(context.Background(), &CategoryListParams{}); return err }},
		{"category list bad category", func() error {
			_, err := c.File.CategoryList(context.Background(), &CategoryListParams{Category: "9"})
			return err
		}},
		{"category list bad order", func() error {
			_, err := c.File.CategoryList(context.Background(), &CategoryListParams{Category: "4", Order: Ptr("bad")})
			return err
		}},
		{"category list invalid combination", func() error {
			_, err := c.File.CategoryList(context.Background(), &CategoryListParams{Category: "4", Recursion: Ptr(1), ShowDir: Ptr(1)})
			return err
		}},
		{"query nil", func() error { _, err := c.File.QueryFileInfo(context.Background(), nil); return err }},
		{"query empty fsids", func() error { _, err := c.File.QueryFileInfo(context.Background(), &QueryFileInfoParams{}); return err }},
		{"query too many fsids", func() error {
			_, err := c.File.QueryFileInfo(context.Background(), &QueryFileInfoParams{FsIDs: make([]int64, 101)})
			return err
		}},
		{"query nonpositive fsid", func() error {
			_, err := c.File.QueryFileInfo(context.Background(), &QueryFileInfoParams{FsIDs: []int64{0}})
			return err
		}},
		{"query bad flag", func() error {
			_, err := c.File.QueryFileInfo(context.Background(), &QueryFileInfoParams{FsIDs: []int64{1}, Dlink: Ptr(2)})
			return err
		}},
		{"keyword nil", func() error { _, err := c.File.KeywordSearch(context.Background(), nil); return err }},
		{"keyword empty", func() error { _, err := c.File.KeywordSearch(context.Background(), &KeywordSearchParams{}); return err }},
		{"keyword too long", func() error {
			_, err := c.File.KeywordSearch(context.Background(), &KeywordSearchParams{Key: "1234567890123456789012345678901"})
			return err
		}},
		{"keyword bad category", func() error {
			_, err := c.File.KeywordSearch(context.Background(), &KeywordSearchParams{Key: "x", Category: Ptr(8)})
			return err
		}},
		{"keyword bad flag", func() error {
			_, err := c.File.KeywordSearch(context.Background(), &KeywordSearchParams{Key: "x", Web: Ptr(2)})
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("validation sent %d requests", got)
	}
}

func TestFileInfo_APIErrorAndContextCancellation(t *testing.T) {
	t.Run("api error", func(t *testing.T) {
		ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"errno":-6,"errmsg":"denied"}`))
		})
		defer ts.Close()
		_, err := c.File.ListAll(context.Background(), &ListAllParams{Path: "/docs"})
		if !IsErrno(err, -6) {
			t.Fatalf("error = %v, want errno -6", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
			<-r.Context().Done()
		})
		defer ts.Close()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := c.File.ListAll(ctx, &ListAllParams{Path: "/docs"})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	})

}

func assertFileInfo(t *testing.T, info *FileInfo) {
	t.Helper()
	if info == nil || info.FsID != 123 || info.Path != "/docs/readme.pdf" || info.ServerFilename != "readme.pdf" || info.Filename != "readme.pdf" || info.Size != 1024 || info.Category != 4 || info.Dlink == "" || info.Height != 720 || info.Width != 1280 || info.MediaInfo["duration"] != float64(4) || info.Privacy != 1 || info.ServerAtime != 12 || info.ExtentTinyint1 != 2 {
		t.Fatalf("decoded file info = %+v", info)
	}
}
