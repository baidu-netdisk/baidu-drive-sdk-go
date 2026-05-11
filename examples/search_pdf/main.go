// examples/search_pdf/main.go — 测试搜索场景：只获取 PDF 文件
//
// 使用方式:
//
//	BAIDU_ACCESS_TOKEN=xxx go run examples/search_pdf/main.go [query] [dir]
//
// 示例:
//
//	BAIDU_ACCESS_TOKEN=xxx go run examples/search_pdf/main.go "合同" /
//	BAIDU_ACCESS_TOKEN=xxx go run examples/search_pdf/main.go "报告"
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func main() {
	token := os.Getenv("BAIDU_ACCESS_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "请设置环境变量 BAIDU_ACCESS_TOKEN")
		fmt.Fprintln(os.Stderr, "用法: BAIDU_ACCESS_TOKEN=xxx go run examples/search_pdf/main.go [query] [dir]")
		os.Exit(1)
	}

	query := "pdf"
	if len(os.Args) > 1 {
		query = os.Args[1]
	}

	c := api.NewClient(
		api.WithAccessToken(token),
		api.WithDebug(true),
	)
	ctx := context.Background()

	// 构建 dirs 参数：需要 uk + path
	dirs := buildDirs(ctx, c, os.Args)

	fmt.Printf("=== 搜索 PDF 文件 ===\n")
	fmt.Printf("关键词: %s\n", query)
	if len(dirs) > 0 {
		fmt.Printf("目录: %v\n", dirs)
	}
	fmt.Printf("文件类型: category=4 (文档)\n\n")

	params := &api.UniSearchParams{
		Query:    query,
		Scene:    "mcpserver",
		Category: []int{4}, // 4=文档（PDF/DOC/XLS 等）
		Dirs:     dirs,
	}

	resp, err := c.File.UniSearch(ctx, params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "请求失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== 响应信息 ===\n")
	fmt.Printf("error_no:   %d\n", resp.ErrorNo)
	fmt.Printf("error_msg:  %s\n", resp.ErrorMsg)
	fmt.Printf("request_id: %s\n", resp.RequestID.String())
	fmt.Printf("is_end:     %v\n", resp.IsEnd)
	fmt.Printf("分组数:     %d\n", len(resp.Data))

	files := resp.Files()
	fmt.Printf("文件总数:   %d\n", len(files))

	// 统计 PDF 文件 vs 非 PDF 文件
	pdfCount := 0
	nonPdfCount := 0
	for _, f := range files {
		if isPDF(f.Filename) {
			pdfCount++
		} else {
			nonPdfCount++
		}
	}
	fmt.Printf("PDF 文件:   %d\n", pdfCount)
	fmt.Printf("非 PDF 文件: %d\n", nonPdfCount)

	fmt.Printf("\n--- 文件列表 ---\n")
	for i, f := range files {
		tag := "DOC"
		if isPDF(f.Filename) {
			tag = "PDF"
		}
		mtime := time.Unix(f.ServerMtime, 0).Format("2006-01-02 15:04")
		fmt.Printf("%3d. [%s] %s\n", i+1, tag, f.Filename)
		fmt.Printf("     路径: %s\n", f.Path)
		fmt.Printf("     大小: %s  修改: %s  category: %d\n",
			formatSize(f.Size), mtime, f.Category)
		if f.Content != "" {
			content := f.Content
			if len(content) > 100 {
				content = content[:100] + "..."
			}
			fmt.Printf("     内容: %s\n", content)
		}
	}

	if nonPdfCount > 0 {
		fmt.Printf("\n⚠ 注意: category=4 是「文档」类型，包含 PDF/DOC/XLS 等。\n")
		fmt.Printf("  百度网盘 API 没有单独的 PDF category，如需仅 PDF 请在客户端按扩展名过滤。\n")
	}
}

func isPDF(filename string) bool {
	n := len(filename)
	return n > 4 && (filename[n-4:] == ".pdf" || filename[n-4:] == ".PDF")
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// buildDirs 根据命令行参数构建 UniSearchDir 列表。
// 如果指定了目录（args[2]），自动调用 UInfo 获取 uk。
func buildDirs(ctx context.Context, c *api.Client, args []string) []api.UniSearchDir {
	if len(args) <= 2 {
		return nil
	}
	uinfo, _ := c.Nas.UInfo(ctx, nil)
	var uk int64
	if uinfo != nil {
		uk = uinfo.UK
	}
	return []api.UniSearchDir{{UK: uk, Path: args[2]}}
}
