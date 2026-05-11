// 网盘容量查询示例
//
// 使用方式:
//   1. 设置环境变量: export BAIDU_ACCESS_TOKEN=your_access_token
//   2. 运行: go run main.go
//
// 获取 access_token 的方式:
//   - 参考 examples/auth/main.go 进行 OAuth2 授权
//   - 或在百度网盘开放平台获取: https://pan.baidu.com/union

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func main() {
	if err := run(os.Getenv, os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(getenv func(string) string, args []string) error {
	accessToken := getenv("BAIDU_ACCESS_TOKEN")
	if accessToken == "" {
		return fmt.Errorf("错误: 请设置 BAIDU_ACCESS_TOKEN 环境变量\n示例: export BAIDU_ACCESS_TOKEN=your_access_token")
	}

	// 创建客户端
	client := api.NewClient(api.WithAccessToken(accessToken))

	return runWithClient(client)
}

func runWithClient(client *api.Client) error {
	fmt.Println("正在查询网盘容量信息...")
	fmt.Println()

	// 调用 Quota 接口
	ctx := context.Background()
	resp, err := client.Nas.Quota(ctx, nil)
	if err != nil {
		return fmt.Errorf("查询失败: %v", err)
	}

	// 格式化输出结果
	fmt.Println("========================================")
	fmt.Println("          网盘容量信息")
	fmt.Println("========================================")
	fmt.Printf("总容量:   %s (%d bytes)\n", formatBytes(resp.Total), resp.Total)
	fmt.Printf("已使用:   %s (%d bytes)\n", formatBytes(resp.Used), resp.Used)
	fmt.Printf("剩余可用: %s (%d bytes)\n", formatBytes(resp.Total-resp.Used), resp.Total-resp.Used)
	fmt.Printf("使用率:   %.2f%%\n", float64(resp.Used)*100/float64(resp.Total))
	fmt.Println("----------------------------------------")
	if resp.Expire {
		fmt.Println("⚠️  提醒: 7天内有容量即将到期")
	} else {
		fmt.Println("✓ 7天内无容量到期")
	}
	fmt.Println("========================================")
	return nil
}

// formatBytes 将字节数转换为人类可读的格式
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
