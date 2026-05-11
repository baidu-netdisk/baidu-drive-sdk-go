package scene

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// =============================================================================
// file_manager.go 测试
// =============================================================================

// mockRouter 根据请求路径和参数分发到不同的 handler。
type mockRouter struct {
	t        *testing.T
	handlers map[string]http.HandlerFunc
}

func (m *mockRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Path
	if method := r.URL.Query().Get("method"); method != "" {
		key += "?" + method
	}
	if opera := r.URL.Query().Get("opera"); opera != "" {
		key += "&opera=" + opera
	}
	if h, ok := m.handlers[key]; ok {
		h(w, r)
		return
	}
	m.t.Errorf("unhandled request: %s %s", r.Method, r.URL.String())
	w.WriteHeader(http.StatusNotFound)
}

func listHandler(files string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0,"list":` + files + `,"request_id":1,"guid":0}`))
	}
}

// listByDirHandler 根据 dir 参数返回不同的文件列表。
func listByDirHandler(dirFiles map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := r.URL.Query().Get("dir")
		files, ok := dirFiles[dir]
		if !ok {
			files = `[]`
		}
		w.Write([]byte(`{"errno":0,"list":` + files + `,"request_id":1,"guid":0}`))
	}
}

// listPathNotExistHandler 返回 errno=-9 表示路径不存在。
func listPathNotExistHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-9,"errmsg":"path not exist"}`))
	}
}

func manageOKHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0,"info":[],"request_id":1}`))
	}
}

func mkdirOKHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0,"fs_id":1,"category":6,"path":"/test","ctime":1,"mtime":1,"isdir":1,"status":0}`))
	}
}

func listErrorHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}
}

func newRouterScene(t *testing.T, handlers map[string]http.HandlerFunc) (*httptest.Server, *Scene) {
	router := &mockRouter{t: t, handlers: handlers}
	ts := httptest.NewServer(router)
	c := api.NewClient(api.WithBaseURL(ts.URL))
	return ts, New(c)
}

const existingFile = `[{"fs_id":1,"path":"/dst/a.txt","server_filename":"a.txt","size":100,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":0,"category":4,"md5":"abc"}]`

const srcFile = `[{"fs_id":2,"path":"/src/a.txt","server_filename":"a.txt","size":100,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":0,"category":4,"md5":"def"}]`

// =============================================================================
// CopyFile 测试
// =============================================================================

func TestScene_CopyFile(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": srcFile,
			"/dst": `[]`,
		}),
		"/rest/2.0/xpan/file?filemanager&opera=copy": manageOKHandler(),
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScene_CopyFile_AlreadyExists(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": srcFile,
			"/dst": existingFile,
		}),
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !errors.Is(err, ErrFileAlreadyExist) {
		t.Errorf("expected ErrFileAlreadyExist, got: %v", err)
	}
}

func TestScene_CopyFile_ListError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listErrorHandler(),
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for List API failure")
	}
}

// =============================================================================
// CopyFileWithOndup 测试
// =============================================================================

func TestScene_CopyFileWithOndup_Newcopy(t *testing.T) {
	var capturedOndup string
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": srcFile,
			// /dst 目录已有同名文件，但 newcopy 时不做预检查
			"/dst": existingFile,
		}),
		"/rest/2.0/xpan/file?filemanager&opera=copy": func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			capturedOndup = r.FormValue("ondup")
			w.Write([]byte(`{"errno":0,"info":[],"request_id":1}`))
		},
	})
	defer ts.Close()

	err := sc.CopyFileWithOndup(context.Background(), "/src/a.txt", "/dst", "a.txt", "newcopy")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOndup != "newcopy" {
		t.Errorf("ondup sent to API = %q, want newcopy", capturedOndup)
	}
}

// =============================================================================
// MoveFile 测试
// =============================================================================

func TestScene_MoveFile(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": srcFile,
			"/dst": `[]`,
		}),
		"/rest/2.0/xpan/file?filemanager&opera=move": manageOKHandler(),
	})
	defer ts.Close()

	err := sc.MoveFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScene_MoveFile_AlreadyExists(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": srcFile,
			"/dst": existingFile,
		}),
	})
	defer ts.Close()

	err := sc.MoveFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !errors.Is(err, ErrFileAlreadyExist) {
		t.Errorf("expected ErrFileAlreadyExist, got: %v", err)
	}
}

func TestScene_MoveFile_ListError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listErrorHandler(),
	})
	defer ts.Close()

	err := sc.MoveFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for List API failure")
	}
}

// =============================================================================
// RenameFile 测试
// =============================================================================

func TestScene_RenameFile(t *testing.T) {
	oldFile := `[{"fs_id":3,"path":"/test/old.txt","server_filename":"old.txt","size":100,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":0,"category":4,"md5":"abc"}]`
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list":                    listHandler(oldFile),
		"/rest/2.0/xpan/file?filemanager&opera=rename": manageOKHandler(),
	})
	defer ts.Close()

	err := sc.RenameFile(context.Background(), "/test/old.txt", "new.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScene_RenameFile_AlreadyExists(t *testing.T) {
	// 目录中同时存在 old.txt 和 new.txt
	existing := `[{"fs_id":3,"path":"/test/old.txt","server_filename":"old.txt","size":100,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":0,"category":4,"md5":"abc"},{"fs_id":1,"path":"/test/new.txt","server_filename":"new.txt","size":100,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":0,"category":4,"md5":"abc"}]`
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listHandler(existing),
	})
	defer ts.Close()

	err := sc.RenameFile(context.Background(), "/test/old.txt", "new.txt")
	if err == nil {
		t.Fatal("expected error for existing file")
	}
	if !errors.Is(err, ErrFileAlreadyExist) {
		t.Errorf("expected ErrFileAlreadyExist, got: %v", err)
	}
}

func TestScene_RenameFile_ListError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listErrorHandler(),
	})
	defer ts.Close()

	err := sc.RenameFile(context.Background(), "/test/old.txt", "new.txt")
	if err == nil {
		t.Fatal("expected error for List API failure")
	}
}

// =============================================================================
// 源文件不存在测试
// =============================================================================

func TestScene_CopyFile_SrcNotExist(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": `[]`, // 源目录为空
		}),
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
	if !errors.Is(err, ErrFileNotExist) {
		t.Errorf("expected ErrFileNotExist, got: %v", err)
	}
}

func TestScene_MoveFile_SrcNotExist(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": `[]`,
		}),
	})
	defer ts.Close()

	err := sc.MoveFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
	if !errors.Is(err, ErrFileNotExist) {
		t.Errorf("expected ErrFileNotExist, got: %v", err)
	}
}

func TestScene_RenameFile_SrcNotExist(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listHandler(`[]`), // 目录为空
	})
	defer ts.Close()

	err := sc.RenameFile(context.Background(), "/test/old.txt", "new.txt")
	if err == nil {
		t.Fatal("expected error for non-existent source")
	}
	if !errors.Is(err, ErrFileNotExist) {
		t.Errorf("expected ErrFileNotExist, got: %v", err)
	}
}

// =============================================================================
// 目标目录不存在（errno=-9 容错）测试
// =============================================================================

func TestScene_CopyFile_DestDirNotExist(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			dir := r.URL.Query().Get("dir")
			if dir == "/src" {
				w.Write([]byte(`{"errno":0,"list":` + srcFile + `,"request_id":1,"guid":0}`))
			} else {
				// 目标目录不存在
				w.Write([]byte(`{"errno":-9,"errmsg":"path not exist"}`))
			}
		},
		"/rest/2.0/xpan/file?filemanager&opera=copy": manageOKHandler(),
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/src/a.txt", "/nonexist", "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v (dest dir not exist should not block pre-check)", err)
	}
}

func TestScene_MoveFile_DestDirNotExist(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			dir := r.URL.Query().Get("dir")
			if dir == "/src" {
				w.Write([]byte(`{"errno":0,"list":` + srcFile + `,"request_id":1,"guid":0}`))
			} else {
				w.Write([]byte(`{"errno":-9,"errmsg":"path not exist"}`))
			}
		},
		"/rest/2.0/xpan/file?filemanager&opera=move": manageOKHandler(),
	})
	defer ts.Close()

	err := sc.MoveFile(context.Background(), "/src/a.txt", "/nonexist", "a.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v (dest dir not exist should not block pre-check)", err)
	}
}

func TestScene_CopyFile_SrcDirNotExist(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listPathNotExistHandler(),
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/nonexist/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for non-existent source dir")
	}
	if !errors.Is(err, ErrFileNotExist) {
		t.Errorf("expected ErrFileNotExist, got: %v", err)
	}
}

// =============================================================================
// MkdirIfNotExist 测试
// =============================================================================

func TestScene_MkdirIfNotExist_NotExists(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list":   listHandler(`[]`),
		"/rest/2.0/xpan/file?create": mkdirOKHandler(),
	})
	defer ts.Close()

	err := sc.MkdirIfNotExist(context.Background(), "/test/newdir")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScene_MkdirIfNotExist_AlreadyExists(t *testing.T) {
	existing := `[{"fs_id":1,"path":"/test/newdir","server_filename":"newdir","size":0,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":1,"category":6,"md5":""}]`
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listHandler(existing),
	})
	defer ts.Close()

	err := sc.MkdirIfNotExist(context.Background(), "/test/newdir")
	if err != nil {
		t.Fatalf("unexpected error: %v (should succeed silently)", err)
	}
}

func TestScene_MkdirIfNotExist_ListError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listErrorHandler(),
	})
	defer ts.Close()

	err := sc.MkdirIfNotExist(context.Background(), "/test/newdir")
	if err == nil {
		t.Fatal("expected error for List API failure")
	}
}

// =============================================================================
// ListDir 测试
// =============================================================================

func TestScene_ListDir(t *testing.T) {
	files := `[
		{"fs_id":1,"path":"/test/a.txt","server_filename":"a.txt","size":1024,"server_mtime":100,"server_ctime":90,"local_mtime":100,"local_ctime":90,"isdir":0,"category":4,"md5":"abc"},
		{"fs_id":2,"path":"/test/subdir","server_filename":"subdir","size":0,"server_mtime":200,"server_ctime":180,"local_mtime":200,"local_ctime":180,"isdir":1,"category":6,"md5":""}
	]`
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listHandler(files),
	})
	defer ts.Close()

	results, err := sc.ListDir(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Filename != "a.txt" {
		t.Errorf("results[0].Filename = %q, want a.txt", results[0].Filename)
	}
	if results[0].IsDir {
		t.Error("results[0].IsDir should be false")
	}
	if results[0].Size != 1024 {
		t.Errorf("results[0].Size = %d, want 1024", results[0].Size)
	}
	if results[0].MD5 != "abc" {
		t.Errorf("results[0].MD5 = %q, want abc", results[0].MD5)
	}
	if results[1].Filename != "subdir" {
		t.Errorf("results[1].Filename = %q, want subdir", results[1].Filename)
	}
	if !results[1].IsDir {
		t.Error("results[1].IsDir should be true")
	}
}

func TestScene_ListDir_DefaultRoot(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			if dir := r.URL.Query().Get("dir"); dir != "/" {
				t.Errorf("dir = %q, want /", dir)
			}
			w.Write([]byte(`{"errno":0,"list":[],"request_id":1,"guid":0}`))
		},
	})
	defer ts.Close()

	results, err := sc.ListDir(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestScene_ListDir_APIError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listErrorHandler(),
	})
	defer ts.Close()

	_, err := sc.ListDir(context.Background(), "/test", nil)
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

// =============================================================================
// ListDir with Options 测试
// =============================================================================

func TestScene_ListDir_WithOptions(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
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
			if q.Get("folder") != "1" {
				t.Errorf("folder = %q, want 1", q.Get("folder"))
			}
			w.Write([]byte(`{"errno":0,"list":[],"request_id":1,"guid":0}`))
		},
	})
	defer ts.Close()

	opts := &ListDirOptions{
		Order:      "time",
		Desc:       true,
		Start:      10,
		Limit:      50,
		FolderOnly: true,
	}
	results, err := sc.ListDir(context.Background(), "/test", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("len(results) = %d, want 0", len(results))
	}
}

func TestScene_ListDir_NilOptions(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			// nil options should not set optional params
			if q.Get("order") != "" {
				t.Errorf("order should be empty, got %q", q.Get("order"))
			}
			w.Write([]byte(`{"errno":0,"list":[],"request_id":1,"guid":0}`))
		},
	})
	defer ts.Close()

	_, err := sc.ListDir(context.Background(), "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// =============================================================================
// DeleteFile 测试
// =============================================================================

func TestScene_DeleteFile(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?filemanager&opera=delete": manageOKHandler(),
	})
	defer ts.Close()

	err := sc.DeleteFile(context.Background(), []string{"/test/a.txt", "/test/b.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScene_DeleteFile_EmptyPaths(t *testing.T) {
	sc := New(nil) // no client needed, should return nil immediately
	err := sc.DeleteFile(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScene_DeleteFile_APIError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?filemanager&opera=delete": listErrorHandler(),
	})
	defer ts.Close()

	err := sc.DeleteFile(context.Background(), []string{"/test/a.txt"})
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}

// =============================================================================
// fileExistsInDir 返回非 ErrnoPathNotExist 错误的测试
// =============================================================================

func TestScene_CopyFile_DestDirListGenericError(t *testing.T) {
	// src exists, but dest dir list returns a generic error (not -9)
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			dir := r.URL.Query().Get("dir")
			if dir == "/src" {
				w.Write([]byte(`{"errno":0,"list":` + srcFile + `,"request_id":1,"guid":0}`))
			} else {
				// Generic error (not -9) for dest dir
				w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
			}
		},
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for generic dest dir list failure")
	}
	if !strings.Contains(err.Error(), "copy check") {
		t.Errorf("error = %q, want to contain 'copy check'", err.Error())
	}
}

func TestScene_MoveFile_DestDirListGenericError(t *testing.T) {
	// src exists, but dest dir list returns a generic error (not -9)
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			dir := r.URL.Query().Get("dir")
			if dir == "/src" {
				w.Write([]byte(`{"errno":0,"list":` + srcFile + `,"request_id":1,"guid":0}`))
			} else {
				w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
			}
		},
	})
	defer ts.Close()

	err := sc.MoveFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for generic dest dir list failure")
	}
	if !strings.Contains(err.Error(), "move check") {
		t.Errorf("error = %q, want to contain 'move check'", err.Error())
	}
}

func TestScene_RenameFile_DirListGenericError(t *testing.T) {
	// First call for src check succeeds (file exists), second call for dest check fails
	callCount := 0
	oldFile := `[{"fs_id":3,"path":"/test/old.txt","server_filename":"old.txt","size":100,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":0,"category":4,"md5":"abc"}]`
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				// First call: src check — file exists
				w.Write([]byte(`{"errno":0,"list":` + oldFile + `,"request_id":1,"guid":0}`))
			} else {
				// Second call: dest check — generic error
				w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
			}
		},
	})
	defer ts.Close()

	err := sc.RenameFile(context.Background(), "/test/old.txt", "new.txt")
	if err == nil {
		t.Fatal("expected error for generic dir list failure")
	}
	if !strings.Contains(err.Error(), "rename check") {
		t.Errorf("error = %q, want to contain 'rename check'", err.Error())
	}
}

// =============================================================================
// MkdirIfNotExist — mkdir API error 测试
// =============================================================================

func TestScene_MkdirIfNotExist_MkdirAPIError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listHandler(`[]`), // dir not exist (empty list)
		"/rest/2.0/xpan/file?create": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"errno":-8,"errmsg":"file already exists"}`))
		},
	})
	defer ts.Close()

	err := sc.MkdirIfNotExist(context.Background(), "/test/newdir")
	if err == nil {
		t.Fatal("expected error for mkdir API failure")
	}
	if !strings.Contains(err.Error(), "scene: mkdir") {
		t.Errorf("error = %q, want to contain 'scene: mkdir'", err.Error())
	}
}

// =============================================================================
// Copy/Move/Rename API error 测试
// =============================================================================

func TestScene_CopyFile_CopyAPIError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": srcFile,
			"/dst": `[]`,
		}),
		"/rest/2.0/xpan/file?filemanager&opera=copy": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"errno":-9,"errmsg":"path not found"}`))
		},
	})
	defer ts.Close()

	err := sc.CopyFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for copy API failure")
	}
	if !strings.Contains(err.Error(), "scene: copy") {
		t.Errorf("error = %q, want to contain 'scene: copy'", err.Error())
	}
}

func TestScene_MoveFile_MoveAPIError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list": listByDirHandler(map[string]string{
			"/src": srcFile,
			"/dst": `[]`,
		}),
		"/rest/2.0/xpan/file?filemanager&opera=move": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"errno":-9,"errmsg":"path not found"}`))
		},
	})
	defer ts.Close()

	err := sc.MoveFile(context.Background(), "/src/a.txt", "/dst", "a.txt")
	if err == nil {
		t.Fatal("expected error for move API failure")
	}
	if !strings.Contains(err.Error(), "scene: move") {
		t.Errorf("error = %q, want to contain 'scene: move'", err.Error())
	}
}

func TestScene_RenameFile_RenameAPIError(t *testing.T) {
	oldFile := `[{"fs_id":3,"path":"/test/old.txt","server_filename":"old.txt","size":100,"server_mtime":1,"server_ctime":1,"local_mtime":1,"local_ctime":1,"isdir":0,"category":4,"md5":"abc"}]`
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/file?list":                    listHandler(oldFile),
		"/rest/2.0/xpan/file?filemanager&opera=rename": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"errno":-9,"errmsg":"path not found"}`))
		},
	})
	defer ts.Close()

	err := sc.RenameFile(context.Background(), "/test/old.txt", "new.txt")
	if err == nil {
		t.Fatal("expected error for rename API failure")
	}
	if !strings.Contains(err.Error(), "scene: rename") {
		t.Errorf("error = %q, want to contain 'scene: rename'", err.Error())
	}
}
