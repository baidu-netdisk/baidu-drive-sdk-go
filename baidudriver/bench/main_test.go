package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene"
)

func TestParseFileSize(t *testing.T) {
	tests := []struct {
		input   string
		want    int64
		wantErr bool
	}{
		{"1KB", 1024, false},
		{"1kb", 1024, false},
		{"1MB", 1024 * 1024, false},
		{"10MB", 10 * 1024 * 1024, false},
		{"100MB", 100 * 1024 * 1024, false},
		{" 1mb ", 1024 * 1024, false},
		{"2GB", 0, true},
		{"", 0, true},
		{"abc", 0, true},
	}
	for _, tt := range tests {
		got, err := parseFileSize(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseFileSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseFileSize(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestAvg(t *testing.T) {
	tests := []struct {
		vals []float64
		want float64
	}{
		{nil, 0},
		{[]float64{}, 0},
		{[]float64{10}, 10},
		{[]float64{10, 20, 30}, 20},
		{[]float64{1, 2, 3, 4}, 2.5},
	}
	for _, tt := range tests {
		got := avg(tt.vals)
		if got != tt.want {
			t.Errorf("avg(%v) = %f, want %f", tt.vals, got, tt.want)
		}
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		sorted []float64
		p      float64
		want   float64
	}{
		{nil, 50, 0},
		{[]float64{}, 50, 0},
		{[]float64{100}, 50, 100},
		{[]float64{100}, 0, 100},
		{[]float64{100}, 100, 100},
		{[]float64{10, 20, 30, 40, 50}, 0, 10},
		{[]float64{10, 20, 30, 40, 50}, 100, 50},
		{[]float64{10, 20, 30, 40, 50}, 50, 30},
	}
	for _, tt := range tests {
		got := percentile(tt.sorted, tt.p)
		if got != tt.want {
			t.Errorf("percentile(%v, %f) = %f, want %f", tt.sorted, tt.p, got, tt.want)
		}
	}

	got := percentile([]float64{10, 20, 30, 40, 50}, 25)
	if got != 20 {
		t.Errorf("percentile P25 = %f, want 20", got)
	}

	got = percentile([]float64{10, 20, 30, 40, 50}, 75)
	if got != 40 {
		t.Errorf("percentile P75 = %f, want 40", got)
	}
}

func TestPrintResults(t *testing.T) {
	results := []*benchResult{
		{Duration: 100 * time.Millisecond, Bytes: 1024},
		{Duration: 200 * time.Millisecond, Bytes: 1024},
		{Duration: 150 * time.Millisecond, Bytes: 1024},
	}
	printResults("Upload", results, 1024)

	errResults := []*benchResult{
		{Duration: 10 * time.Millisecond, Err: errForTest("err1")},
		{Duration: 20 * time.Millisecond, Err: errForTest("err2")},
	}
	printResults("Download", errResults, 1024)

	mixedResults := []*benchResult{
		{Duration: 100 * time.Millisecond, Bytes: 1024},
		{Duration: 50 * time.Millisecond, Err: errForTest("timeout")},
	}
	printResults("Mixed", mixedResults, 1024)

	printResults("Empty", []*benchResult{}, 1024)
}

func TestPrintResults_LongError(t *testing.T) {
	longErr := ""
	for len(longErr) < 100 {
		longErr += "e"
	}
	results := []*benchResult{
		{Duration: 10 * time.Millisecond, Err: errForTest(longErr)},
	}
	printResults("LongErr", results, 1024)
}

func TestPrintResults_ZeroDuration(t *testing.T) {
	results := []*benchResult{
		{Duration: 0, Bytes: 1024},
	}
	printResults("Zero", results, 1024)
}

func TestSummarizeResults(t *testing.T) {
	results := []*benchResult{
		{Duration: 100 * time.Millisecond, Bytes: 100},
		{Duration: 300 * time.Millisecond, Err: errForTest("rate limited")},
		{Duration: 200 * time.Millisecond, Bytes: 100},
	}
	report := summarizeResults(results)
	if report.Requests != 3 || report.Successes != 2 || report.Failures != 1 {
		t.Fatalf("summarizeResults counts = %+v", report)
	}
	if report.P50MS != 150 || report.P95MS != 195 || report.P99MS != 199 {
		t.Fatalf("summarizeResults percentiles = %+v", report)
	}
	if report.ThroughputRPS <= 0 || report.Errors["rate limited"] != 1 {
		t.Fatalf("summarizeResults metrics = %+v", report)
	}
}

func TestWriteBenchmarkReport(t *testing.T) {
	path := t.TempDir() + "/result.json"
	want := benchmarkReport{Requests: 2, Successes: 2, P50MS: 1.5}
	if err := writeBenchmarkReport(path, want); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got benchmarkReport
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Requests != want.Requests || got.Successes != want.Successes || got.P50MS != want.P50MS {
		t.Fatalf("written benchmark report = %+v", got)
	}
}

type errForTest string

func (e errForTest) Error() string { return string(e) }

func TestRunBenchmark(t *testing.T) {
	var callCount int64
	results := runBenchmark(t.Context(), 2, 5, func(i int) *benchResult {
		atomic.AddInt64(&callCount, 1)
		return &benchResult{Duration: 10 * time.Millisecond, Bytes: 100}
	})
	if len(results) != 5 {
		t.Errorf("len(results) = %d, want 5", len(results))
	}
	if got := atomic.LoadInt64(&callCount); got != 5 {
		t.Errorf("callCount = %d, want 5", got)
	}
}

func TestRunBenchmark_WithErrors(t *testing.T) {
	results := runBenchmark(t.Context(), 1, 3, func(i int) *benchResult {
		if i == 1 {
			return &benchResult{Duration: 5 * time.Millisecond, Err: errForTest("fail")}
		}
		return &benchResult{Duration: 10 * time.Millisecond, Bytes: 100}
	})
	if len(results) != 3 {
		t.Errorf("len(results) = %d, want 3", len(results))
	}
	if results[1].Err == nil {
		t.Error("results[1] should have error")
	}
}

func TestRun_RejectsInvalidOptions(t *testing.T) {
	origMode, origCount, origConcurrency := *mode, *count, *concurrency
	defer func() {
		*mode = origMode
		*count = origCount
		*concurrency = origConcurrency
	}()

	client := api.NewClient()
	sc := scene.New(client)
	for _, tt := range []struct {
		name string
		set  func()
		want string
	}{
		{name: "mode", set: func() { *mode = "invalid" }, want: "invalid mode"},
		{name: "concurrency", set: func() { *mode, *concurrency = "upload", 0 }, want: "concurrency"},
		{name: "count", set: func() { *mode, *concurrency, *count = "upload", 1, 0 }, want: "count"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			*mode, *count, *concurrency = "both", 1, 1
			tt.set()
			if err := run(context.Background(), sc, client, 1); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("run error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBenchQuota_Success(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	origMode, origCount, origConc := *mode, *count, *concurrency
	*mode, *count, *concurrency = "quota", 3, 2
	defer func() { *mode, *count, *concurrency = origMode, origCount, origConc }()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(c)
	if err := run(context.Background(), sc, c, 1); err != nil {
		t.Fatalf("quota benchmark failed: %v", err)
	}
}

// =============================================================================
// benchUpload / benchDownload / prepareDownloadFile / cleanupFile 测试
// =============================================================================

// newBenchTestServer 创建一个模拟百度网盘 API 的测试服务器。
func newBenchTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := r.URL.Query().Get("method")
		switch method {
		case "precreate":
			w.Write([]byte(`{"errno":0,"uploadid":"test-uid","block_list":[0],"return_type":1}`))
		case "upload":
			w.Write([]byte(`{"errno":0,"md5":"testmd5"}`))
		case "create":
			w.Write([]byte(`{"errno":0,"fs_id":12345,"path":"/apps/test/file.bin","size":1024,"md5":"testmd5"}`))
		case "filemetas":
			w.Write([]byte(fmt.Sprintf(`{"errno":0,"list":[{"fs_id":12345,"path":"/apps/test/file.bin","filename":"file.bin","size":1024,"md5":"testmd5","dlink":"%s/download?param=1"}]}`, serverURL)))
		case "filemanager":
			w.Write([]byte(`{"errno":0,"info":[],"request_id":1}`))
		default:
			// Download endpoint
			if r.URL.Path == "/download" {
				w.Header().Set("Content-Length", "5")
				w.Write([]byte("hello"))
				return
			}
			w.Write([]byte(`{"errno":0}`))
		}
	}))
	serverURL = ts.URL
	return ts
}

func TestBenchUpload_Success(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	result := benchUpload(context.Background(), sc, c, 1024, 0)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Bytes != 1024 {
		t.Errorf("Bytes = %d, want 1024", result.Bytes)
	}
	if result.Duration <= 0 {
		t.Error("Duration should be positive")
	}

	// 等待异步清理 goroutine
	time.Sleep(50 * time.Millisecond)
}

func TestBenchUpload_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	result := benchUpload(context.Background(), sc, c, 100, 0)
	if result.Err == nil {
		t.Fatal("expected error for API failure")
	}
	if result.Bytes != 100 {
		t.Errorf("Bytes = %d, want 100", result.Bytes)
	}
}

func TestBenchDownload_Success(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(c)

	result := benchDownload(context.Background(), sc, c, 12345, 0)
	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Bytes <= 0 {
		t.Error("Bytes should be positive")
	}
}

func TestBenchDownload_APIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(c)

	result := benchDownload(context.Background(), sc, c, 12345, 0)
	if result.Err == nil {
		t.Fatal("expected error for API failure")
	}
}

func TestBenchFileInfo_AllOperations(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	origPath, origFSID := *fileInfoPath, *fileInfoFsID
	*fileInfoPath, *fileInfoFsID = "/apps/test", 12345
	defer func() { *fileInfoPath, *fileInfoFsID = origPath, origFSID }()

	for i := 0; i < 11; i++ {
		result := benchFileInfo(context.Background(), c, 12345, i)
		if result.Err != nil {
			t.Fatalf("operation %d failed: %v", i, result.Err)
		}
		if result.Duration < 0 {
			t.Fatalf("operation %d duration = %v", i, result.Duration)
		}
	}
}

func TestRun_FileInfoMode(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(c)
	origMode, origCount, origConcurrency, origPath, origFSID := *mode, *count, *concurrency, *fileInfoPath, *fileInfoFsID
	*mode, *count, *concurrency, *fileInfoPath, *fileInfoFsID = "file-info", 11, 2, "/apps/test", 12345
	defer func() {
		*mode, *count, *concurrency, *fileInfoPath, *fileInfoFsID = origMode, origCount, origConcurrency, origPath, origFSID
	}()

	if err := run(context.Background(), sc, c, 1); err != nil {
		t.Fatalf("file-info run failed: %v", err)
	}
}

func TestPrepareDownloadFile_Success(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	fsID := prepareDownloadFile(context.Background(), sc, 1024)
	if fsID != 12345 {
		t.Errorf("fsID = %d, want 12345", fsID)
	}
}

func TestPrepareDownloadFile_UploadError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	fsID := prepareDownloadFile(context.Background(), sc, 100)
	if fsID != 0 {
		t.Errorf("fsID = %d, want 0 for error case", fsID)
	}
}

func TestCleanupFile_Success(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(c)

	// 不 panic 即可
	cleanupFile(context.Background(), sc, 12345)
}

func TestCleanupFile_MetaError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(c)

	// 不应 panic，meta 失败时直接 return
	cleanupFile(context.Background(), sc, 99999)
}

func TestCleanupFile_EmptyList(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0,"list":[]}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(c)

	// meta 返回空列表时不应 panic
	cleanupFile(context.Background(), sc, 99999)
}

// =============================================================================
// run 函数测试
// =============================================================================

func TestRun_UploadMode(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	origMode, origCount, origConc := *mode, *count, *concurrency
	*mode = "upload"
	*count = 2
	*concurrency = 1
	defer func() {
		*mode = origMode
		*count = origCount
		*concurrency = origConc
	}()

	err := run(context.Background(), sc, c, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_WritesIterationLoadResult(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	resultPath := t.TempDir() + "/load-result.json"
	t.Setenv("ITERATION_LOAD_RESULT", resultPath)
	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	origMode, origCount, origConc := *mode, *count, *concurrency
	*mode = "upload"
	*count = 2
	*concurrency = 1
	defer func() {
		*mode = origMode
		*count = origCount
		*concurrency = origConc
	}()

	if err := run(context.Background(), sc, c, 100); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	raw, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var report benchmarkReport
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if report.Requests != 2 || report.Successes != 2 || report.Failures != 0 {
		t.Fatalf("iteration load result = %+v", report)
	}
}

func TestRun_DownloadMode(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	origMode, origCount, origConc := *mode, *count, *concurrency
	*mode = "download"
	*count = 2
	*concurrency = 1
	defer func() {
		*mode = origMode
		*count = origCount
		*concurrency = origConc
	}()

	err := run(context.Background(), sc, c, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_BothMode(t *testing.T) {
	ts := newBenchTestServer(t)
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	origMode, origCount, origConc := *mode, *count, *concurrency
	*mode = "both"
	*count = 1
	*concurrency = 1
	defer func() {
		*mode = origMode
		*count = origCount
		*concurrency = origConc
	}()

	err := run(context.Background(), sc, c, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_DownloadPrepareError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL), api.WithPCSBaseURL(ts.URL))
	sc := scene.New(c)

	origMode, origCount, origConc := *mode, *count, *concurrency
	*mode = "download"
	*count = 1
	*concurrency = 1
	defer func() {
		*mode = origMode
		*count = origCount
		*concurrency = origConc
	}()

	err := run(context.Background(), sc, c, 100)
	if err == nil {
		t.Fatal("expected error for prepare failure")
	}
}

// =============================================================================
// main 相关辅助测试
// =============================================================================

func TestParseFileSize_Invalid(t *testing.T) {
	_, err := parseFileSize("invalid")
	if err == nil {
		t.Error("expected error")
	}
}
