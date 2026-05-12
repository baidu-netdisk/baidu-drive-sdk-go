package scene

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// newTestScene 创建测试用的 Scene 实例。
func newTestScene(handler http.HandlerFunc) (*httptest.Server, *Scene) {
	ts := httptest.NewServer(handler)
	c := api.NewClient(api.WithBaseURL(ts.URL))
	return ts, New(c)
}

// =============================================================================
// Client 测试
// =============================================================================

func TestScene_Client(t *testing.T) {
	_, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {})
	c := sc.Client()
	if c == nil {
		t.Fatal("Client() returned nil")
	}
}

// =============================================================================
// Search 测试
// =============================================================================

func TestScene_Search(t *testing.T) {
	uinfoCallCount := 0
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/2.0/xpan/nas":
			uinfoCallCount++
			w.Write([]byte(`{"errno":0,"uk":2871298235,"baidu_name":"test","netdisk_name":"test"}`))
		case "/xpan/unisearch":
			if r.Method != http.MethodPost {
				t.Errorf("method = %q, want POST", r.Method)
			}
			// 验证参数在 query string
			q := r.URL.Query()
			if q.Get("query") != "document" {
				t.Errorf("query = %q, want document", q.Get("query"))
			}
			if q.Get("dirs") != `[{"uk":2871298235,"path":"/docs"}]` {
				t.Errorf("dirs = %q, want %q", q.Get("dirs"), `[{"uk":2871298235,"path":"/docs"}]`)
			}

			// 返回真实嵌套结构 data[].list[]
			w.Write([]byte(`{
				"data": [
					{
						"source": 1,
						"list": [
							{
								"category": 4,
								"filename": "document.pdf",
								"fsid": 12345,
								"isdir": 0,
								"path": "/docs/document.pdf",
								"parent_path": "/docs",
								"content": "匹配的关键内容",
								"server_ctime": 1670242019,
								"server_mtime": 1670242019,
								"size": 10240
							},
							{
								"category": 6,
								"filename": "mydir",
								"fsid": 67890,
								"isdir": 1,
								"path": "/docs/mydir",
								"parent_path": "/docs",
								"server_ctime": 1670242000,
								"server_mtime": 1670242000,
								"size": 0
							}
						]
					}
				],
				"error_no": 0,
				"is_end": true,
				"request_id": 1,
				"server_time": 1670242019
			}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	})
	defer ts.Close()

	results, err := sc.Search(context.Background(), &SearchParams{
		Query: "document",
		Dir:   "/docs",
		Num:   100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	// 验证第一个文件
	if results[0].Filename != "document.pdf" {
		t.Errorf("results[0].Filename = %q, want document.pdf", results[0].Filename)
	}
	if results[0].Path != "/docs/document.pdf" {
		t.Errorf("results[0].Path = %q, want /docs/document.pdf", results[0].Path)
	}
	if results[0].IsDir {
		t.Error("results[0].IsDir should be false for file")
	}
	if results[0].Content != "匹配的关键内容" {
		t.Errorf("results[0].Content = %q, want 匹配的关键内容", results[0].Content)
	}

	// 验证第二个目录
	if results[1].Filename != "mydir" {
		t.Errorf("results[1].Filename = %q, want mydir", results[1].Filename)
	}
	if !results[1].IsDir {
		t.Error("results[1].IsDir should be true for directory")
	}
}

func TestScene_Search_MinimalParams(t *testing.T) {
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"data": [],
			"error_no": 0,
			"is_end": true,
			"request_id": 0,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	results, err := sc.Search(context.Background(), &SearchParams{
		Query: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestScene_Search_NilParams(t *testing.T) {
	_, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {})

	_, err := sc.Search(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestScene_Search_EmptyQuery(t *testing.T) {
	_, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {})

	_, err := sc.Search(context.Background(), &SearchParams{
		Query: "",
		Dir:   "/test",
	})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestScene_Search_APIError(t *testing.T) {
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error_no":-6,"error_msg":"access denied"}`))
	})
	defer ts.Close()

	_, err := sc.Search(context.Background(), &SearchParams{
		Query: "test",
	})
	if err == nil {
		t.Fatal("expected error for API error")
	}
}

func TestScene_Search_MultipleSourceGroups(t *testing.T) {
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		// 多个 source 分组，验证 Files() 展平
		w.Write([]byte(`{
			"data": [
				{
					"source": 1,
					"list": [
						{"category": 4, "filename": "a.pdf", "fsid": 1, "isdir": 0, "path": "/a.pdf", "server_ctime": 1, "server_mtime": 1, "size": 100}
					]
				},
				{
					"source": 2,
					"list": [
						{"category": 3, "filename": "b.png", "fsid": 2, "isdir": 0, "path": "/b.png", "server_ctime": 1, "server_mtime": 1, "size": 200}
					]
				}
			],
			"error_no": 0,
			"is_end": true,
			"request_id": 123,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	results, err := sc.Search(context.Background(), &SearchParams{
		Query: "test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 两个分组各一个文件，展平后应该有 2 个结果
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Filename != "a.pdf" {
		t.Errorf("results[0].Filename = %q, want a.pdf", results[0].Filename)
	}
	if results[1].Filename != "b.png" {
		t.Errorf("results[1].Filename = %q, want b.png", results[1].Filename)
	}
}

func TestScene_Search_WithNum(t *testing.T) {
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"data": [
				{
					"source": 1,
					"list": [
						{"category": 4, "filename": "a.pdf", "fsid": 1, "isdir": 0, "path": "/a.pdf", "server_ctime": 1, "server_mtime": 1, "size": 100},
						{"category": 4, "filename": "b.pdf", "fsid": 2, "isdir": 0, "path": "/b.pdf", "server_ctime": 1, "server_mtime": 1, "size": 100},
						{"category": 4, "filename": "c.pdf", "fsid": 3, "isdir": 0, "path": "/c.pdf", "server_ctime": 1, "server_mtime": 1, "size": 100}
					]
				}
			],
			"error_no": 0,
			"is_end": true,
			"request_id": 456,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	results, err := sc.Search(context.Background(), &SearchParams{
		Query: "pdf",
		Num:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3", len(results))
	}
}

func TestScene_Search_CategoryFilter(t *testing.T) {
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// 验证 category 参数正确透传（4=文档，PDF 属于文档类型）
		if q.Get("category") != "[4]" {
			t.Errorf("category = %q, want [4]", q.Get("category"))
		}
		if q.Get("query") != "pdf" {
			t.Errorf("query = %q, want pdf", q.Get("query"))
		}

		// 模拟返回 PDF 文件
		w.Write([]byte(`{
			"data": [
				{
					"source": 1,
					"list": [
						{
							"category": 4,
							"filename": "report.pdf",
							"fsid": 100,
							"isdir": 0,
							"path": "/docs/report.pdf",
							"parent_path": "/docs",
							"server_ctime": 1670242019,
							"server_mtime": 1670242019,
							"size": 204800
						},
						{
							"category": 4,
							"filename": "invoice.pdf",
							"fsid": 101,
							"isdir": 0,
							"path": "/docs/invoice.pdf",
							"parent_path": "/docs",
							"server_ctime": 1670242020,
							"server_mtime": 1670242020,
							"size": 102400
						}
					]
				}
			],
			"error_no": 0,
			"is_end": true,
			"request_id": 789,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	results, err := sc.Search(context.Background(), &SearchParams{
		Query:    "pdf",
		Category: []int{4},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	// 验证所有结果都是文档类型
	for i, r := range results {
		if r.Category != 4 {
			t.Errorf("results[%d].Category = %d, want 4", i, r.Category)
		}
	}
	if results[0].Filename != "report.pdf" {
		t.Errorf("results[0].Filename = %q, want report.pdf", results[0].Filename)
	}
	if results[1].Filename != "invoice.pdf" {
		t.Errorf("results[1].Filename = %q, want invoice.pdf", results[1].Filename)
	}
}

func TestScene_Search_UKCaching(t *testing.T) {
	uinfoCallCount := 0
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/2.0/xpan/nas":
			uinfoCallCount++
			w.Write([]byte(`{"errno":0,"uk":2871298235,"baidu_name":"test","netdisk_name":"test"}`))
		case "/xpan/unisearch":
			w.Write([]byte(`{"data":[],"error_no":0,"is_end":true,"request_id":1,"server_time":1}`))
		}
	})
	defer ts.Close()

	// 第一次搜索
	_, err := sc.Search(context.Background(), &SearchParams{
		Query: "test1",
		Dir:   "/docs",
	})
	if err != nil {
		t.Fatalf("first search: %v", err)
	}
	// 第二次搜索
	_, err = sc.Search(context.Background(), &SearchParams{
		Query: "test2",
		Dir:   "/docs",
	})
	if err != nil {
		t.Fatalf("second search: %v", err)
	}
	// UInfo 应该只调用一次
	if uinfoCallCount != 1 {
		t.Errorf("uinfo call count = %d, want 1", uinfoCallCount)
	}
}

func TestScene_Search_UInfoError(t *testing.T) {
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/2.0/xpan/nas":
			w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
		case "/xpan/unisearch":
			t.Error("unisearch should not be called when UInfo fails")
		}
	})
	defer ts.Close()

	_, err := sc.Search(context.Background(), &SearchParams{
		Query: "test",
		Dir:   "/docs",
	})
	if err == nil {
		t.Fatal("expected error when UInfo fails")
	}
}

func TestScene_Search_UInfoRetry(t *testing.T) {
	uinfoCallCount := 0
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/2.0/xpan/nas":
			uinfoCallCount++
			if uinfoCallCount == 1 {
				// 第一次失败
				w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
			} else {
				// 第二次成功
				w.Write([]byte(`{"errno":0,"uk":2871298235,"baidu_name":"test","netdisk_name":"test"}`))
			}
		case "/xpan/unisearch":
			w.Write([]byte(`{"data":[],"error_no":0,"is_end":true,"request_id":1,"server_time":1}`))
		}
	})
	defer ts.Close()

	// 第一次搜索应失败（UInfo 失败）
	_, err := sc.Search(context.Background(), &SearchParams{
		Query: "test",
		Dir:   "/docs",
	})
	if err == nil {
		t.Fatal("expected error on first search")
	}

	// 第二次搜索应成功（UInfo 重试成功）
	_, err = sc.Search(context.Background(), &SearchParams{
		Query: "test",
		Dir:   "/docs",
	})
	if err != nil {
		t.Fatalf("second search should succeed: %v", err)
	}
	if uinfoCallCount != 2 {
		t.Errorf("uinfo call count = %d, want 2", uinfoCallCount)
	}
}

func TestScene_Search_UInfoReturnsZeroUK(t *testing.T) {
	ts, sc := newTestScene(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/2.0/xpan/nas":
			// UInfo 成功但 uk=0
			w.Write([]byte(`{"errno":0,"uk":0,"baidu_name":"test","netdisk_name":"test"}`))
		case "/xpan/unisearch":
			t.Error("unisearch should not be called when uk=0")
		}
	})
	defer ts.Close()

	_, err := sc.Search(context.Background(), &SearchParams{
		Query: "test",
		Dir:   "/docs",
	})
	if err == nil {
		t.Fatal("expected error when uk=0")
	}
	if !strings.Contains(err.Error(), "uk=0") {
		t.Errorf("error should mention uk=0, got: %v", err)
	}
}
