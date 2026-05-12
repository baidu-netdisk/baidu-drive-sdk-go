// examples/file_list/main.go — 验证 File.List limit 参数是否生效
//
// 使用方式:
//
//	BAIDU_ACCESS_TOKEN=xxx go run examples/file_list/main.go [dir] [limit]
//
// 示例:
//
//	BAIDU_ACCESS_TOKEN=xxx go run examples/file_list/main.go / 5
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func main() {
	token := os.Getenv("BAIDU_ACCESS_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "请设置环境变量 BAIDU_ACCESS_TOKEN")
		fmt.Fprintln(os.Stderr, "用法: BAIDU_ACCESS_TOKEN=xxx go run examples/file_list/main.go [dir] [limit]")
		os.Exit(1)
	}

	dir := "/"
	limit := 5
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if len(os.Args) > 2 {
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "limit 参数必须为整数: %v\n", err)
			os.Exit(1)
		}
		limit = v
	}

	// 开启 debug 模式，打印完整请求 URL 和响应体
	c := api.NewClient(
		api.WithAccessToken(token),
		api.WithDebug(true),
	)
	ctx := context.Background()

	fmt.Printf("=== 测试 File.List limit 参数 ===\n")
	fmt.Printf("目录: %s\n", dir)
	fmt.Printf("请求 limit: %d\n\n", limit)

	resp, err := c.File.List(ctx, &api.ListParams{
		Dir:   dir,
		Start: api.Ptr(0),
		Limit: api.Ptr(limit),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "请求失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== 结果 ===\n")
	fmt.Printf("errno: %d\n", resp.Errno)
	fmt.Printf("实际返回文件数: %d\n", len(resp.List))
	fmt.Printf("请求 limit: %d\n", limit)

	if len(resp.List) > limit {
		fmt.Printf("\n⚠ 服务端忽略了 limit 参数！返回了 %d 个文件（期望 ≤ %d）\n", len(resp.List), limit)
	} else {
		fmt.Printf("\n✓ limit 参数生效，返回数量符合预期\n")
	}

	fmt.Printf("\n--- 文件列表 ---\n")
	for i, f := range resp.List {
		typ := "文件"
		if f.Isdir == 1 {
			typ = "目录"
		}
		fmt.Printf("%3d. [%s] %s (%s)\n", i+1, typ, f.ServerFilename, f.Path)
	}
}
