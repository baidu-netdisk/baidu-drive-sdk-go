// Command bench 对百度网盘 SDK 上传/下载、文件信息只读接口或 Quota 查询进行基准测试。
//
// 用法:
//
//	# BDPAN_ACCESS_TOKEN is injected by cmd/iteration after directory OAuth.
//	BDPAN_ACCESS_TOKEN=xxx go run ./baidudriver/bench \
//	    -mode upload \
//	    -concurrency 5 \
//	    -filesize 1MB \
//	    -count 10
//
// 参数:
//
//	-mode          测试模式: upload, download, both, quota, file-info (默认 both)
//	-concurrency   并发数 (默认 1)
//	-filesize      文件大小: 1KB, 1MB, 10MB, 100MB (默认 1MB)
//	-count         每种模式执行次数 (默认 10)
//	-impl          实现: new (默认, 使用 scene 层), api (仅 api 层，无重试)
//	-remote-dir    远程目录 (默认 /apps/bdpan_sdk_bench)
//	-file-info-path 文件信息只读测试目录 (默认 /apps/bdpan测试)
//	-file-info-fsid 文件信息查询接口使用的 fs_id；不指定时从测试目录读取一个文件
//	-result-file   写入机器可读结果 JSON；未设置时读取 ITERATION_LOAD_RESULT
package main

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene"
)

var (
	mode         = flag.String("mode", "both", "test mode: upload, download, both, quota, file-info")
	concurrency  = flag.Int("concurrency", 1, "concurrency level")
	filesize     = flag.String("filesize", "1MB", "file size: 1KB, 1MB, 10MB, 100MB")
	count        = flag.Int("count", 10, "iterations per mode")
	impl         = flag.String("impl", "new", "implementation: new (scene layer), api (raw api)")
	remoteDir    = flag.String("remote-dir", "/apps/bdpan_sdk_bench", "remote directory")
	fileInfoPath = flag.String("file-info-path", "/apps/bdpan测试", "read-only file-info benchmark path")
	fileInfoFsID = flag.Int64("file-info-fsid", 0, "fs_id for file-info meta benchmark (0 discovers one)")
	resultFile   = flag.String("result-file", "", "write machine-readable result JSON (or use ITERATION_LOAD_RESULT)")
)

func main() {
	flag.Parse()

	token := os.Getenv("BDPAN_ACCESS_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "Error: BDPAN_ACCESS_TOKEN environment variable required")
		os.Exit(1)
	}

	size, err := parseFileSize(*filesize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	client := api.NewClient(api.WithAccessToken(token))
	sc := scene.New(client)

	if err := run(context.Background(), sc, client, size); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// run 执行基准测试的核心逻辑。
func run(ctx context.Context, sc *scene.Scene, client *api.Client, size int64) error {
	switch *mode {
	case "upload", "download", "both", "quota", "file-info":
	default:
		return fmt.Errorf("invalid mode %q: want upload, download, both, quota, or file-info", *mode)
	}
	if *concurrency < 1 {
		return fmt.Errorf("concurrency must be at least 1")
	}
	if *count < 1 {
		return fmt.Errorf("count must be at least 1")
	}
	fmt.Printf("=== Baidu Pan SDK Benchmark ===\n")
	fmt.Printf("Mode:        %s\n", *mode)
	fmt.Printf("Impl:        %s\n", *impl)
	fmt.Printf("Concurrency: %d\n", *concurrency)
	fmt.Printf("File Size:   %s (%d bytes)\n", *filesize, size)
	fmt.Printf("Count:       %d\n", *count)
	fmt.Printf("Remote Dir:  %s\n", *remoteDir)
	fmt.Println(strings.Repeat("=", 50))

	var allResults []*benchResult
	var benchmarkElapsed time.Duration
	if *mode == "file-info" {
		fsID, err := resolveFileInfoFsID(ctx, client)
		if err != nil {
			return err
		}
		fmt.Printf("File Info Path: %s\n", *fileInfoPath)
		if fsID > 0 {
			fmt.Printf("File Info FsID:  %d\n", fsID)
		} else {
			fmt.Println("File Info FsID:  not found (filemetas requests may fail)")
		}
		fmt.Println("\n--- File Info Benchmark (11 read-only APIs, round-robin) ---")
		results, elapsed := runBenchmarkMeasured(ctx, *concurrency, *count, func(i int) *benchResult {
			return benchFileInfo(ctx, client, fsID, i)
		})
		printResults("File Info", results, 0)
		allResults = append(allResults, results...)
		benchmarkElapsed = elapsed
	}
	if *mode == "quota" {
		fmt.Println("\n--- Quota Benchmark (read-only) ---")
		results, elapsed := runBenchmarkMeasured(ctx, *concurrency, *count, func(i int) *benchResult {
			return benchQuota(ctx, client)
		})
		printResults("Quota", results, 0)
		allResults = append(allResults, results...)
		benchmarkElapsed = elapsed
	}
	if *mode == "upload" || *mode == "both" {
		fmt.Println("\n--- Upload Benchmark ---")
		results, elapsed := runBenchmarkMeasured(ctx, *concurrency, *count, func(i int) *benchResult {
			return benchUpload(ctx, sc, client, size, i)
		})
		printResults("Upload", results, size)
		allResults = append(allResults, results...)
		benchmarkElapsed += elapsed
	}

	if *mode == "download" || *mode == "both" {
		fmt.Println("\n--- Download Benchmark ---")
		fsID := prepareDownloadFile(ctx, sc, size)
		if fsID == 0 {
			return fmt.Errorf("failed to prepare download file")
		}
		defer cleanupFile(ctx, sc, fsID)

		results, elapsed := runBenchmarkMeasured(ctx, *concurrency, *count, func(i int) *benchResult {
			return benchDownload(ctx, sc, client, fsID, i)
		})
		printResults("Download", results, size)
		allResults = append(allResults, results...)
		benchmarkElapsed += elapsed
	}
	if path := benchmarkResultPath(); path != "" {
		if err := writeBenchmarkReport(path, summarizeResultsWithElapsed(allResults, benchmarkElapsed)); err != nil {
			return err
		}
	}
	return nil
}

func benchQuota(ctx context.Context, client *api.Client) *benchResult {
	start := time.Now()
	_, err := client.Nas.Quota(ctx, nil)
	return &benchResult{Duration: time.Since(start), Err: err}
}

// resolveFileInfoFsID performs one read-only list request before the benchmark.
// The result is reused so every filemetas call has a deterministic target.
func resolveFileInfoFsID(ctx context.Context, client *api.Client) (int64, error) {
	if *fileInfoFsID > 0 {
		return *fileInfoFsID, nil
	}
	if raw := strings.TrimSpace(os.Getenv("BDPAN_FILE_INFO_FSID")); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("BDPAN_FILE_INFO_FSID must be a positive integer: %q", raw)
		}
		return parsed, nil
	}
	resp, err := client.File.List(ctx, &api.ListParams{Dir: *fileInfoPath, Start: api.Ptr(0), Limit: api.Ptr(1)})
	if err != nil {
		return 0, fmt.Errorf("resolve file-info fsid: %w", err)
	}
	for _, file := range resp.List {
		if file != nil && file.FsID > 0 && file.Isdir == 0 {
			return file.FsID, nil
		}
	}
	return 0, nil
}

// benchFileInfo rotates over the eleven read-only APIs in the file-info menu.
// It deliberately does not create, modify, or delete remote data.
func benchFileInfo(ctx context.Context, client *api.Client, fsID int64, index int) *benchResult {
	type operation struct {
		name string
		call func() error
	}
	operations := []operation{
		{"list", func() error {
			_, err := client.File.List(ctx, &api.ListParams{Dir: *fileInfoPath, Start: api.Ptr(0), Limit: api.Ptr(100)})
			return err
		}},
		{"listall", func() error {
			_, err := client.File.ListAll(ctx, &api.ListAllParams{Path: *fileInfoPath, Recursion: api.Ptr(1), Start: api.Ptr(0), Limit: api.Ptr(100)})
			return err
		}},
		{"doclist", func() error {
			_, err := client.File.DocumentList(ctx, &api.DocumentListParams{ParentPath: *fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(100)})
			return err
		}},
		{"imagelist", func() error {
			_, err := client.File.ImageList(ctx, &api.ImageListParams{ParentPath: *fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(100)})
			return err
		}},
		{"videolist", func() error {
			_, err := client.File.VideoList(ctx, &api.VideoListParams{ParentPath: *fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(100)})
			return err
		}},
		{"btlist", func() error {
			_, err := client.File.BTList(ctx, &api.BTListParams{ParentPath: *fileInfoPath, Page: api.Ptr(1), Num: api.Ptr(100)})
			return err
		}},
		{"categoryinfo", func() error {
			_, err := client.File.CategoryCount(ctx, &api.CategoryCountParams{Category: 4, ParentPath: *fileInfoPath})
			return err
		}},
		{"categorylist", func() error {
			_, err := client.File.CategoryList(ctx, &api.CategoryListParams{Category: "4", ParentPath: *fileInfoPath, Start: api.Ptr(0), Limit: api.Ptr(100)})
			return err
		}},
		{"filemetas", func() error {
			if fsID <= 0 {
				return errors.New("no file fs_id available; pass -file-info-fsid")
			}
			_, err := client.File.QueryFileInfo(ctx, &api.QueryFileInfoParams{FsIDs: []int64{fsID}})
			return err
		}},
		{"search", func() error {
			_, err := client.File.KeywordSearch(ctx, &api.KeywordSearchParams{Key: "README", Dir: *fileInfoPath})
			return err
		}},
		{"unisearch", func() error {
			_, err := client.File.UniSearch(ctx, &api.UniSearchParams{Query: "README", Scene: "mcpserver", Num: api.Ptr(100)})
			return err
		}},
	}
	op := operations[index%len(operations)]
	start := time.Now()
	if err := op.call(); err != nil {
		return &benchResult{Duration: time.Since(start), Err: fmt.Errorf("%s: %w", op.name, err)}
	}
	return &benchResult{Duration: time.Since(start)}
}

type benchmarkReport struct {
	Requests      int            `json:"requests"`
	Successes     int            `json:"successes"`
	Failures      int            `json:"failures"`
	P50MS         float64        `json:"p50_ms"`
	P95MS         float64        `json:"p95_ms"`
	P99MS         float64        `json:"p99_ms"`
	ThroughputRPS float64        `json:"throughput_rps"`
	Errors        map[string]int `json:"errors,omitempty"`
}

type benchResult struct {
	Duration time.Duration
	Err      error
	Bytes    int64
}

func benchmarkResultPath() string {
	if strings.TrimSpace(*resultFile) != "" {
		return strings.TrimSpace(*resultFile)
	}
	return strings.TrimSpace(os.Getenv("ITERATION_LOAD_RESULT"))
}

func summarizeResults(results []*benchResult) benchmarkReport {
	return summarizeResultsWithElapsed(results, 0)
}

func summarizeResultsWithElapsed(results []*benchResult, elapsed time.Duration) benchmarkReport {
	report := benchmarkReport{Requests: len(results), Errors: make(map[string]int)}
	var durations []float64
	var totalDuration time.Duration
	for _, result := range results {
		if result == nil {
			report.Failures++
			report.Errors["nil result"]++
			continue
		}
		totalDuration += result.Duration
		if result.Err != nil {
			report.Failures++
			errKey := redactBench(result.Err.Error())
			if len(errKey) > 80 {
				errKey = errKey[:80]
			}
			report.Errors[errKey]++
			continue
		}
		report.Successes++
		durations = append(durations, result.Duration.Seconds()*1000)
	}
	sort.Float64s(durations)
	report.P50MS = percentile(durations, 50)
	report.P95MS = percentile(durations, 95)
	report.P99MS = percentile(durations, 99)
	if elapsed > 0 {
		report.ThroughputRPS = float64(report.Successes) / elapsed.Seconds()
	} else if totalDuration > 0 {
		// Used by direct callers that do not have the worker wall time.
		report.ThroughputRPS = float64(report.Successes) / totalDuration.Seconds()
	}
	if len(report.Errors) == 0 {
		report.Errors = nil
	}
	return report
}

func writeBenchmarkReport(path string, report benchmarkReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write benchmark result: %w", err)
	}
	return nil
}

func redactBench(value string) string {
	for _, name := range []string{"BDPAN_ACCESS_TOKEN", "BAIDU_ACCESS_TOKEN"} {
		if token := os.Getenv(name); token != "" {
			value = strings.ReplaceAll(value, token, "***")
		}
	}
	return value
}

func runBenchmark(ctx context.Context, conc, total int, fn func(int) *benchResult) []*benchResult {
	results, _ := runBenchmarkMeasured(ctx, conc, total, fn)
	return results
}

func runBenchmarkMeasured(ctx context.Context, conc, total int, fn func(int) *benchResult) ([]*benchResult, time.Duration) {
	results := make([]*benchResult, total)
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	start := time.Now()
	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = fn(idx)
			status := "OK"
			if results[idx] == nil {
				status = "ERR: nil result"
			} else if results[idx].Err != nil {
				status = fmt.Sprintf("ERR: %v", results[idx].Err)
			}
			var duration time.Duration
			if results[idx] != nil {
				duration = results[idx].Duration
			}
			fmt.Printf("  [%d/%d] %v %s\n", idx+1, total, duration.Round(time.Millisecond), status)
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("  Total wall time: %v\n", elapsed.Round(time.Millisecond))

	return results, elapsed
}

func benchUpload(ctx context.Context, sc *scene.Scene, client *api.Client, size int64, idx int) *benchResult {
	content := make([]byte, size)
	rand.Read(content)

	tmpFile, err := os.CreateTemp("", "bench_upload_*")
	if err != nil {
		return &benchResult{Err: err}
	}
	tmpFile.Write(content)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	h := md5.Sum(content)
	remotePath := fmt.Sprintf("%s/bench_%s_%d.bin", *remoteDir, hex.EncodeToString(h[:8]), idx)

	start := time.Now()
	result, err := sc.UploadFile(ctx, &scene.UploadFileParams{
		LocalPath:  tmpFile.Name(),
		RemotePath: remotePath,
		RType:      api.Ptr(1),
	})
	elapsed := time.Since(start)

	if err != nil {
		return &benchResult{Duration: elapsed, Err: err, Bytes: size}
	}

	// Cleanup
	go sc.DeleteFile(context.Background(), []string{result.Path})

	return &benchResult{Duration: elapsed, Bytes: size}
}

func benchDownload(ctx context.Context, sc *scene.Scene, client *api.Client, fsID int64, idx int) *benchResult {
	tmpFile, err := os.CreateTemp("", "bench_download_*")
	if err != nil {
		return &benchResult{Err: err}
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	start := time.Now()
	result, err := sc.DownloadFile(ctx, &scene.DownloadFileParams{
		FsID:      fsID,
		LocalPath: tmpFile.Name(),
	})
	elapsed := time.Since(start)

	if err != nil {
		return &benchResult{Duration: elapsed, Err: err}
	}

	return &benchResult{Duration: elapsed, Bytes: result.Size}
}

func prepareDownloadFile(ctx context.Context, sc *scene.Scene, size int64) int64 {
	content := make([]byte, size)
	rand.Read(content)

	tmpFile, err := os.CreateTemp("", "bench_prep_*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare: create temp: %v\n", err)
		return 0
	}
	tmpFile.Write(content)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	h := md5.Sum(content)
	remotePath := fmt.Sprintf("%s/bench_download_%s.bin", *remoteDir, hex.EncodeToString(h[:8]))

	fmt.Printf("  Preparing download file (%d bytes)...\n", size)
	result, err := sc.UploadFile(ctx, &scene.UploadFileParams{
		LocalPath:  tmpFile.Name(),
		RemotePath: remotePath,
		RType:      api.Ptr(1),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare: upload: %v\n", err)
		return 0
	}
	fmt.Printf("  Prepared: fsid=%d path=%s\n", result.FsID, result.Path)
	return result.FsID
}

func cleanupFile(ctx context.Context, sc *scene.Scene, fsID int64) {
	// Get meta to find path
	resp, err := sc.Client().Download.Meta(ctx, &api.MetaParams{FsIDs: []int64{fsID}})
	if err != nil || len(resp.List) == 0 {
		return
	}
	sc.DeleteFile(ctx, []string{resp.List[0].Path})
}

func printResults(label string, results []*benchResult, fileSize int64) {
	var durations []float64
	successCount := 0
	errorCounts := make(map[string]int)

	for _, r := range results {
		if r.Err != nil {
			errKey := r.Err.Error()
			if len(errKey) > 80 {
				errKey = errKey[:80]
			}
			errorCounts[errKey]++
			continue
		}
		successCount++
		durations = append(durations, r.Duration.Seconds()*1000) // ms
	}

	total := len(results)
	fmt.Printf("\n%s Results:\n", label)
	fmt.Printf("  Success Rate: %d/%d (%.1f%%)\n", successCount, total, float64(successCount)/float64(total)*100)

	if len(durations) > 0 {
		sort.Float64s(durations)
		fmt.Printf("  Latency (ms):\n")
		fmt.Printf("    P50:  %.0f\n", percentile(durations, 50))
		fmt.Printf("    P95:  %.0f\n", percentile(durations, 95))
		fmt.Printf("    P99:  %.0f\n", percentile(durations, 99))
		fmt.Printf("    Min:  %.0f\n", durations[0])
		fmt.Printf("    Max:  %.0f\n", durations[len(durations)-1])
		fmt.Printf("    Avg:  %.0f\n", avg(durations))

		avgDurationSec := avg(durations) / 1000
		if fileSize > 0 && avgDurationSec > 0 {
			throughputMBps := float64(fileSize) / (1024 * 1024) / avgDurationSec
			fmt.Printf("  Throughput: %.2f MB/s (avg)\n", throughputMBps)
		}
	}

	if len(errorCounts) > 0 {
		fmt.Printf("  Error Classification:\n")
		for errMsg, cnt := range errorCounts {
			fmt.Printf("    [%d] %s\n", cnt, errMsg)
		}
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := p / 100 * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))
	if lower == upper || upper >= len(sorted) {
		return sorted[lower]
	}
	frac := idx - float64(lower)
	return sorted[lower]*(1-frac) + sorted[upper]*frac
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func parseFileSize(s string) (int64, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	switch s {
	case "1KB":
		return 1024, nil
	case "1MB":
		return 1024 * 1024, nil
	case "10MB":
		return 10 * 1024 * 1024, nil
	case "100MB":
		return 100 * 1024 * 1024, nil
	default:
		return 0, fmt.Errorf("unsupported file size %q, use: 1KB, 1MB, 10MB, 100MB", s)
	}
}
