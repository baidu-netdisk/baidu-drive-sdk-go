package api

import (
	"context"
	"io"
	"net/http"
	"testing"
)

// =============================================================================
// UniSearch 测试
// =============================================================================

func TestFile_UniSearch(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/xpan/unisearch" {
			t.Errorf("path = %q, want /xpan/unisearch", r.URL.Path)
		}

		// 验证参数在 query string 中
		q := r.URL.Query()
		if q.Get("query") != "test" {
			t.Errorf("query param = %q, want test", q.Get("query"))
		}
		if q.Get("scene") != "mcpserver" {
			t.Errorf("scene param = %q, want mcpserver", q.Get("scene"))
		}
		if q.Get("dirs") != `[{"uk":123,"path":"/test"}]` {
			t.Errorf("dirs param = %q, want %q", q.Get("dirs"), `[{"uk":123,"path":"/test"}]`)
		}
		if q.Get("num") != "100" {
			t.Errorf("num param = %q, want 100", q.Get("num"))
		}

		// 验证 body 是空对象
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{}" {
			t.Errorf("body = %q, want {}", string(body))
		}

		// 返回真实嵌套结构: data[].list[]
		w.Write([]byte(`{
			"data": [
				{
					"source": 1,
					"list": [
						{
							"category": 4,
							"filename": "test.pdf",
							"fsid": 12345,
							"isdir": 0,
							"path": "/test/test.pdf",
							"parent_path": "/test",
							"content": "匹配内容",
							"server_ctime": 1670242019,
							"server_mtime": 1670242019,
							"size": 10240
						}
					]
				}
			],
			"error_msg": "",
			"error_no": 0,
			"is_end": true,
			"request_id": 1234567890,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	resp, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "test",
		Scene: "mcpserver",
		Dirs:  []UniSearchDir{{UK: 123, Path: "/test"}},
		Num:   Ptr(100),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ErrorNo != 0 {
		t.Errorf("ErrorNo = %d, want 0", resp.ErrorNo)
	}
	// 验证 request_id 为数字类型
	if resp.RequestID.String() != "1234567890" {
		t.Errorf("RequestID = %q, want 1234567890", resp.RequestID.String())
	}
	// 验证嵌套结构
	if len(resp.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].Source != 1 {
		t.Errorf("Data[0].Source = %d, want 1", resp.Data[0].Source)
	}
	if len(resp.Data[0].List) != 1 {
		t.Fatalf("len(Data[0].List) = %d, want 1", len(resp.Data[0].List))
	}
	if resp.Data[0].List[0].Filename != "test.pdf" {
		t.Errorf("Filename = %q, want test.pdf", resp.Data[0].List[0].Filename)
	}
	if resp.Data[0].List[0].FsID != 12345 {
		t.Errorf("FsID = %d, want 12345", resp.Data[0].List[0].FsID)
	}
	if resp.Data[0].List[0].Content != "匹配内容" {
		t.Errorf("Content = %q, want 匹配内容", resp.Data[0].List[0].Content)
	}
	// 验证 Files() 便捷方法
	files := resp.Files()
	if len(files) != 1 {
		t.Fatalf("len(Files()) = %d, want 1", len(files))
	}
	if files[0].Filename != "test.pdf" {
		t.Errorf("Files()[0].Filename = %q, want test.pdf", files[0].Filename)
	}
}

func TestFile_UniSearch_MultipleGroups(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// 多个 source 分组
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
						{"category": 3, "filename": "b.png", "fsid": 2, "isdir": 0, "path": "/b.png", "server_ctime": 1, "server_mtime": 1, "size": 200},
						{"category": 1, "filename": "c.mp4", "fsid": 3, "isdir": 0, "path": "/c.mp4", "server_ctime": 1, "server_mtime": 1, "size": 300}
					]
				}
			],
			"error_no": 0,
			"is_end": true,
			"request_id": 999,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	resp, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "test",
		Scene: "mcpserver",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(resp.Data))
	}
	// Files() 应展平所有分组
	files := resp.Files()
	if len(files) != 3 {
		t.Fatalf("len(Files()) = %d, want 3", len(files))
	}
	if files[0].Filename != "a.pdf" {
		t.Errorf("files[0].Filename = %q, want a.pdf", files[0].Filename)
	}
	if files[2].Filename != "c.mp4" {
		t.Errorf("files[2].Filename = %q, want c.mp4", files[2].Filename)
	}
}

func TestFile_UniSearch_QueryStringParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// 验证所有参数都在 query string 中
		if q.Get("query") != "video" {
			t.Errorf("query = %q, want video", q.Get("query"))
		}
		if q.Get("scene") != "mcpserver" {
			t.Errorf("scene = %q, want mcpserver", q.Get("scene"))
		}
		if q.Get("dirs") != `[{"uk":123,"path":"/videos"},{"uk":123,"path":"/movies"}]` {
			t.Errorf("dirs = %q, want %q", q.Get("dirs"), `[{"uk":123,"path":"/videos"},{"uk":123,"path":"/movies"}]`)
		}
		if q.Get("category") != "[1,4]" {
			t.Errorf("category = %q, want [1,4]", q.Get("category"))
		}
		if q.Get("num") != "50" {
			t.Errorf("num = %q, want 50", q.Get("num"))
		}
		if q.Get("search_type") != "1" {
			t.Errorf("search_type = %q, want 1", q.Get("search_type"))
		}

		// body 应为空对象
		body, _ := io.ReadAll(r.Body)
		if string(body) != "{}" {
			t.Errorf("body = %q, want {}", string(body))
		}

		w.Write([]byte(`{"data":[],"error_no":0,"is_end":true,"request_id":1,"server_time":1}`))
	})
	defer ts.Close()

	_, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query:      "video",
		Scene:      "mcpserver",
		Dirs:       []UniSearchDir{{UK: 123, Path: "/videos"}, {UK: 123, Path: "/movies"}},
		Category:   []int{1, 4},
		Num:        Ptr(50),
		SearchType: Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFile_UniSearch_MinimalParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("query") != "keyword" {
			t.Errorf("query = %q, want keyword", q.Get("query"))
		}
		// Scene 应默认填充 mcpserver
		if q.Get("scene") != "mcpserver" {
			t.Errorf("scene = %q, want mcpserver", q.Get("scene"))
		}
		// 可选参数不应出现
		if q.Get("dirs") != "" {
			t.Errorf("dirs should be empty, got %q", q.Get("dirs"))
		}

		w.Write([]byte(`{"data":[],"error_no":0,"is_end":true,"request_id":1,"server_time":123}`))
	})
	defer ts.Close()

	resp, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "keyword",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ErrorNo != 0 {
		t.Errorf("ErrorNo = %d, want 0", resp.ErrorNo)
	}
}

func TestFile_UniSearch_NilParams(t *testing.T) {
	c := NewClient()
	_, err := c.File.UniSearch(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestFile_UniSearch_EmptyQuery(t *testing.T) {
	c := NewClient()
	_, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "",
		Scene: "mcpserver",
	})
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestFile_UniSearch_APIError(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error_no":-6,"error_msg":"access denied"}`))
	})
	defer ts.Close()

	_, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "test",
		Scene: "mcpserver",
	})
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}

func TestFile_UniSearch_EmptyResult(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"data": [],
			"error_no": 0,
			"is_end": true,
			"request_id": 0,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	resp, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "nonexistent",
		Scene: "mcpserver",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Data) != 0 {
		t.Errorf("len(Data) = %d, want 0", len(resp.Data))
	}
	if !resp.IsEnd {
		t.Error("IsEnd should be true for empty result")
	}
	files := resp.Files()
	if len(files) != 0 {
		t.Errorf("len(Files()) = %d, want 0", len(files))
	}
}

func TestFile_UniSearch_RequestIDNumeric(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		// API 返回数字类型的 request_id
		w.Write([]byte(`{
			"data": [],
			"error_no": 0,
			"is_end": true,
			"request_id": 9876543210,
			"server_time": 1670242019
		}`))
	})
	defer ts.Close()

	resp, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "test",
		Scene: "mcpserver",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.RequestID.String() != "9876543210" {
		t.Errorf("RequestID = %q, want 9876543210", resp.RequestID.String())
	}
}

func TestFile_UniSearch_WithOCR(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"data": [
				{
					"source": 1,
					"list": [
						{
							"category": 3,
							"filename": "image.png",
							"fsid": 11111,
							"isdir": 0,
							"path": "/images/image.png",
							"ocr": "OCR识别文本内容",
							"server_ctime": 1670242019,
							"server_mtime": 1670242019,
							"size": 5120
						}
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

	resp, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query: "image",
		Scene: "mcpserver",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	files := resp.Files()
	if len(files) != 1 {
		t.Fatalf("len(Files()) = %d, want 1", len(files))
	}
	if files[0].OCR != "OCR识别文本内容" {
		t.Errorf("OCR = %q, want OCR识别文本内容", files[0].OCR)
	}
}

func TestFile_UniSearch_FilesWithNilGroups(t *testing.T) {
	resp := &UniSearchResponse{
		Data: []*UniSearchDataGroup{
			nil,
			{Source: 1, List: []*UniSearchFileInfo{
				{Filename: "a.txt"},
				nil,
				{Filename: "b.txt"},
			}},
			nil,
		},
	}
	files := resp.Files()
	if len(files) != 2 {
		t.Fatalf("len(Files()) = %d, want 2", len(files))
	}
	if files[0].Filename != "a.txt" {
		t.Errorf("files[0].Filename = %q, want a.txt", files[0].Filename)
	}
	if files[1].Filename != "b.txt" {
		t.Errorf("files[1].Filename = %q, want b.txt", files[1].Filename)
	}
}

func TestFile_UniSearch_WithStreamParam(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("stream") != "1" {
			t.Errorf("stream = %q, want 1", q.Get("stream"))
		}
		w.Write([]byte(`{"data":[],"error_no":0,"is_end":true,"request_id":1,"server_time":1}`))
	})
	defer ts.Close()

	_, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query:  "test",
		Scene:  "mcpserver",
		Stream: Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFile_UniSearch_WithSourcesParam(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("sources") != "[1,2]" {
			t.Errorf("sources = %q, want [1,2]", q.Get("sources"))
		}
		w.Write([]byte(`{"data":[],"error_no":0,"is_end":true,"request_id":1,"server_time":1}`))
	})
	defer ts.Close()

	_, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query:   "test",
		Scene:   "mcpserver",
		Sources: []int{1, 2},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFile_UniSearch_AllOptionalParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("stream") != "0" {
			t.Errorf("stream = %q, want 0", q.Get("stream"))
		}
		if q.Get("sources") != "[3]" {
			t.Errorf("sources = %q, want [3]", q.Get("sources"))
		}
		if q.Get("num") != "10" {
			t.Errorf("num = %q, want 10", q.Get("num"))
		}
		if q.Get("search_type") != "2" {
			t.Errorf("search_type = %q, want 2", q.Get("search_type"))
		}
		if q.Get("category") != "[1,3]" {
			t.Errorf("category = %q, want [1,3]", q.Get("category"))
		}
		if q.Get("dirs") != `[{"uk":123,"path":"/test"}]` {
			t.Errorf("dirs = %q, want %q", q.Get("dirs"), `[{"uk":123,"path":"/test"}]`)
		}
		w.Write([]byte(`{"data":[],"error_no":0,"is_end":true,"request_id":1,"server_time":1}`))
	})
	defer ts.Close()

	_, err := c.File.UniSearch(context.Background(), &UniSearchParams{
		Query:      "test",
		Scene:      "mcpserver",
		Dirs:       []UniSearchDir{{UK: 123, Path: "/test"}},
		Category:   []int{1, 3},
		Num:        Ptr(10),
		Stream:     Ptr(0),
		SearchType: Ptr(2),
		Sources:    []int{3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
