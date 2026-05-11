package api

import (
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpload_SliceUpload_Success(t *testing.T) {
	fileContent := "hello world test content"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.URL.Query().Get("method") != "upload" {
			t.Errorf("method param = %q, want upload", r.URL.Query().Get("method"))
		}
		if r.URL.Query().Get("type") != "tmpfile" {
			t.Errorf("type param = %q, want tmpfile", r.URL.Query().Get("type"))
		}
		if r.URL.Query().Get("path") != "/apps/test/file.txt" {
			t.Errorf("path param = %q, want /apps/test/file.txt", r.URL.Query().Get("path"))
		}
		if r.URL.Query().Get("uploadid") != "test-upload-id" {
			t.Errorf("uploadid param = %q, want test-upload-id", r.URL.Query().Get("uploadid"))
		}
		if r.URL.Query().Get("partseq") != "0" {
			t.Errorf("partseq param = %q, want 0", r.URL.Query().Get("partseq"))
		}

		// 验证 Content-Type 是 multipart/form-data
		ct := r.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse Content-Type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("Content-Type = %q, want multipart/form-data", mediaType)
		}

		// 读取 multipart body
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatalf("MultipartReader: %v", err)
		}
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		if part.FormName() != "file" {
			t.Errorf("form field = %q, want file", part.FormName())
		}
		data, _ := io.ReadAll(part)
		if string(data) != fileContent {
			t.Errorf("file content = %q, want %q", string(data), fileContent)
		}

		w.Write([]byte(`{
			"errno": 0,
			"md5": "ab56b4d92b40713acc5af89985d4b786",
			"request_id": 789
		}`))
	}))
	defer ts.Close()

	// SliceUpload 使用 PCS base URL
	c := NewClient(WithPCSBaseURL(ts.URL))
	resp, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		Path:     "/apps/test/file.txt",
		UploadID: "test-upload-id",
		PartSeq:  0,
		File:     strings.NewReader(fileContent),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MD5 != "ab56b4d92b40713acc5af89985d4b786" {
		t.Errorf("MD5 = %q, want ab56b4d92b40713acc5af89985d4b786", resp.MD5)
	}
	if resp.RequestID.String() != "789" {
		t.Errorf("RequestID = %s, want 789", resp.RequestID.String())
	}
}

func TestUpload_SliceUpload_PartSeq2(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("partseq") != "2" {
			t.Errorf("partseq = %q, want 2", r.URL.Query().Get("partseq"))
		}
		w.Write([]byte(`{"errno": 0, "md5": "def456"}`))
	}))
	defer ts.Close()

	c := NewClient(WithPCSBaseURL(ts.URL))
	resp, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		Path:     "/apps/test/big.zip",
		UploadID: "uid",
		PartSeq:  2,
		File:     strings.NewReader("chunk data"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.MD5 != "def456" {
		t.Errorf("MD5 = %q, want def456", resp.MD5)
	}
}

func TestUpload_SliceUpload_NilParams(t *testing.T) {
	c := NewClient(WithPCSBaseURL("https://example.com"))
	_, err := c.Upload.SliceUpload(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for nil params")
	}
}

func TestUpload_SliceUpload_EmptyPath(t *testing.T) {
	c := NewClient(WithPCSBaseURL("https://example.com"))
	_, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		UploadID: "uid",
		File:     strings.NewReader("data"),
	})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestUpload_SliceUpload_EmptyUploadID(t *testing.T) {
	c := NewClient(WithPCSBaseURL("https://example.com"))
	_, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		Path: "/apps/test/file.txt",
		File: strings.NewReader("data"),
	})
	if err == nil {
		t.Fatal("expected error for empty uploadid")
	}
}

func TestUpload_SliceUpload_NilFile(t *testing.T) {
	c := NewClient(WithPCSBaseURL("https://example.com"))
	_, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		Path:     "/apps/test/file.txt",
		UploadID: "uid",
	})
	if err == nil {
		t.Fatal("expected error for nil file")
	}
}

func TestUpload_SliceUpload_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := NewClient(WithPCSBaseURL(ts.URL))
	_, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		Path:     "/apps/test/file.txt",
		UploadID: "uid",
		File:     strings.NewReader("data"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}

func TestUpload_SliceUpload_LargeFile(t *testing.T) {
	// 测试 4MB 分片能正确传输
	size := 4 * 1024 * 1024
	content := strings.Repeat("x", size)

	var receivedSize int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mr, _ := r.MultipartReader()
		part, _ := mr.NextPart()
		data, _ := io.ReadAll(part)
		receivedSize = len(data)
		w.Write([]byte(`{"errno": 0, "md5": "large-file-md5"}`))
	}))
	defer ts.Close()

	c := NewClient(WithPCSBaseURL(ts.URL))
	_, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		Path:     "/apps/test/big.bin",
		UploadID: "uid",
		File:     strings.NewReader(content),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedSize != size {
		t.Errorf("received size = %d, want %d", receivedSize, size)
	}
}

// TestUpload_SliceUpload_MultipartBoundary 验证 Content-Type 中包含正确的 boundary。
func TestUpload_SliceUpload_MultipartBoundary(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse Content-Type: %v", err)
		}
		boundary := params["boundary"]
		if boundary == "" {
			t.Error("missing boundary in Content-Type")
		}

		// 使用 boundary 解析 multipart
		mr := multipart.NewReader(r.Body, boundary)
		part, err := mr.NextPart()
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		if part.FormName() != "file" {
			t.Errorf("field = %q, want file", part.FormName())
		}

		w.Write([]byte(`{"errno": 0, "md5": "boundary-test"}`))
	}))
	defer ts.Close()

	c := NewClient(WithPCSBaseURL(ts.URL))
	_, err := c.Upload.SliceUpload(ctx(), &SliceUploadParams{
		Path:     "/apps/test/file.txt",
		UploadID: "uid",
		File:     strings.NewReader("test"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
