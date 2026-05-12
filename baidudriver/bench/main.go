// Command bench 对百度网盘 SDK 上传/下载进行基准测试。
//
// 用法:
//
//	BDPAN_ACCESS_TOKEN=xxx go run ./baidupan/bench \
//	    -mode upload \
//	    -concurrency 5 \
//	    -filesize 1MB \
//	    -count 10
//
// 参数:
//
//	-mode          测试模式: upload, download, both (默认 both)
//	-concurrency   并发数 (默认 1)
//	-filesize      文件大小: 1KB, 1MB, 10MB, 100MB (默认 1MB)
//	-count         每种模式执行次数 (默认 10)
//	-impl          实现: new (默认, 使用 scene 层), api (仅 api 层，无重试)
//	-remote-dir    远程目录 (默认 /apps/bdpan_sdk_bench)
package main

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene"
)

var (
	mode        = flag.String("mode", "both", "test mode: upload, download, both")
	concurrency = flag.Int("concurrency", 1, "concurrency level")
	filesize    = flag.String("filesize", "1MB", "file size: 1KB, 1MB, 10MB, 100MB")
	count       = flag.Int("count", 10, "iterations per mode")
	impl        = flag.String("impl", "new", "implementation: new (scene layer), api (raw api)")
	remoteDir   = flag.String("remote-dir", "/apps/bdpan_sdk_bench", "remote directory")
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
	fmt.Printf("=== Baidu Pan SDK Benchmark ===\n")
	fmt.Printf("Mode:        %s\n", *mode)
	fmt.Printf("Impl:        %s\n", *impl)
	fmt.Printf("Concurrency: %d\n", *concurrency)
	fmt.Printf("File Size:   %s (%d bytes)\n", *filesize, size)
	fmt.Printf("Count:       %d\n", *count)
	fmt.Printf("Remote Dir:  %s\n", *remoteDir)
	fmt.Println(strings.Repeat("=", 50))

	if *mode == "upload" || *mode == "both" {
		fmt.Println("\n--- Upload Benchmark ---")
		results := runBenchmark(ctx, *concurrency, *count, func(i int) *benchResult {
			return benchUpload(ctx, sc, client, size, i)
		})
		printResults("Upload", results, size)
	}

	if *mode == "download" || *mode == "both" {
		fmt.Println("\n--- Download Benchmark ---")
		fsID := prepareDownloadFile(ctx, sc, size)
		if fsID == 0 {
			return fmt.Errorf("failed to prepare download file")
		}
		defer cleanupFile(ctx, sc, fsID)

		results := runBenchmark(ctx, *concurrency, *count, func(i int) *benchResult {
			return benchDownload(ctx, sc, client, fsID, i)
		})
		printResults("Download", results, size)
	}

	return nil
}

type benchResult struct {
	Duration time.Duration
	Err      error
	Bytes    int64
}

func runBenchmark(ctx context.Context, conc, total int, fn func(int) *benchResult) []*benchResult {
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
			if results[idx].Err != nil {
				status = fmt.Sprintf("ERR: %v", results[idx].Err)
			}
			fmt.Printf("  [%d/%d] %v %s\n", idx+1, total, results[idx].Duration.Round(time.Millisecond), status)
		}(i)
	}
	wg.Wait()
	fmt.Printf("  Total wall time: %v\n", time.Since(start).Round(time.Millisecond))

	return results
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
		durations = append(durations, r.Duration.Seconds() * 1000) // ms
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
		if avgDurationSec > 0 {
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
