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

func TestUpload_Precreate_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Query().Get("method") != "precreate" {
			t.Errorf("method param = %q, want precreate", r.URL.Query().Get("method"))
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}

		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		// 验证必填参数
		if !strings.Contains(bodyStr, "path=%2Fapps%2Ftest%2Ffile.txt") {
			t.Errorf("body should contain path, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "size=1024") {
			t.Errorf("body should contain size=1024, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "isdir=0") {
			t.Errorf("body should contain isdir=0, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "autoinit=1") {
			t.Errorf("body should contain autoinit=1, got: %s", bodyStr)
		}

		w.Write([]byte(`{
			"errno": 0,
			"request_id": 123456,
			"uploadid": "N1-MTAuMjI2LjEuMTM6MTY0OTIxMDM5OToxNzMwMTIwMzE1",
			"block_list": [0],
			"return_type": 1
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path:      "/apps/test/file.txt",
		Size:      1024,
		BlockList: []string{"ab56b4d92b40713acc5af89985d4b786"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadID == "" {
		t.Error("UploadID should not be empty")
	}
	if resp.ReturnType != 1 {
		t.Errorf("ReturnType = %d, want 1", resp.ReturnType)
	}
	if resp.RequestID.String() != "123456" {
		t.Errorf("RequestID = %s, want 123456", resp.RequestID.String())
	}
}

func TestUpload_Precreate_WithOptionalParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		if !strings.Contains(bodyStr, "rtype=3") {
			t.Errorf("body should contain rtype=3, got: %s", bodyStr)
		}
		if !strings.Contains(bodyStr, "is_revision=1") {
			t.Errorf("body should contain is_revision=1, got: %s", bodyStr)
		}

		w.Write([]byte(`{
			"errno": 0,
			"uploadid": "test-upload-id",
			"block_list": [0],
			"return_type": 1
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path:       "/apps/test/file.txt",
		Size:       2048,
		BlockList:  []string{"abc123"},
		RType:      Ptr(3),
		IsRevision: Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.UploadID != "test-upload-id" {
		t.Errorf("UploadID = %q, want test-upload-id", resp.UploadID)
	}
}

func TestUpload_Precreate_RapidUpload(t *testing.T) {
	// return_type=2 表示秒传成功
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"errno": 0,
			"uploadid": "",
			"block_list": [],
			"return_type": 2
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path:      "/apps/test/file.txt",
		Size:      1024,
		BlockList: []string{"abc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ReturnType != 2 {
		t.Errorf("ReturnType = %d, want 2 (rapid upload)", resp.ReturnType)
	}
}

func TestUpload_Precreate_NilParams(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Upload.Precreate(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestUpload_Precreate_EmptyPath(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Size:      1024,
		BlockList: []string{"abc"},
	})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestUpload_Precreate_EmptyBlockList(t *testing.T) {
	c := NewClient(WithBaseURL("https://example.com"))
	_, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path: "/apps/test/file.txt",
		Size: 1024,
	})
	if err == nil {
		t.Fatal("expected error for empty block_list")
	}
}

func TestUpload_Precreate_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path:      "/apps/test/file.txt",
		Size:      1024,
		BlockList: []string{"abc"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}

func TestUpload_Precreate_BlockListSerialization(t *testing.T) {
	// 验证多个分片的 block_list 正确序列化为 JSON 数组
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		// block_list 应该是 URL 编码的 JSON 数组
		vals, _ := url.ParseQuery(bodyStr)
		blockListJSON := vals.Get("block_list")
		var blockList []string
		if err := json.Unmarshal([]byte(blockListJSON), &blockList); err != nil {
			t.Errorf("block_list is not valid JSON: %v, raw: %s", err, blockListJSON)
		}
		if len(blockList) != 3 {
			t.Errorf("block_list length = %d, want 3", len(blockList))
		}

		w.Write([]byte(`{
			"errno": 0,
			"uploadid": "test",
			"block_list": [0, 1, 2],
			"return_type": 1
		}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	resp, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path:      "/apps/test/big.zip",
		Size:      1024 * 1024 * 100,
		BlockList: []string{"abc", "def", "ghi"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.BlockList) != 3 {
		t.Errorf("BlockList length = %d, want 3", len(resp.BlockList))
	}
}

func TestUpload_Precreate_WithAutoinit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("autoinit") != "0" {
			t.Errorf("autoinit = %q, want 0", vals.Get("autoinit"))
		}
		w.Write([]byte(`{"errno":0,"uploadid":"test","block_list":[0],"return_type":1}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path:      "/apps/test/file.txt",
		Size:      1024,
		BlockList: []string{"abc"},
		Autoinit:  Ptr(0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpload_Precreate_WithIsDir(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		if vals.Get("isdir") != "1" {
			t.Errorf("isdir = %q, want 1", vals.Get("isdir"))
		}
		w.Write([]byte(`{"errno":0,"uploadid":"","block_list":[],"return_type":2}`))
	}))
	defer ts.Close()

	c := NewClient(WithBaseURL(ts.URL))
	_, err := c.Upload.Precreate(ctx(), &PrecreateParams{
		Path:      "/apps/test/dir",
		Size:      0,
		BlockList: []string{""},
		IsDir:     Ptr(1),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
