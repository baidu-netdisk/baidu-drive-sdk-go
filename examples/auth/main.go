// examples/auth/main.go — 设备码授权获取 access_token（不存本地）
//
// 使用方式:
//
//	BAIDU_APP_KEY=xxx BAIDU_SECRET_KEY=yyy go run examples/auth/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func main() {
	appKey := os.Getenv("BAIDU_APP_KEY")
	secretKey := os.Getenv("BAIDU_SECRET_KEY")
	if appKey == "" || secretKey == "" {
		fmt.Fprintln(os.Stderr, "请设置环境变量 BAIDU_APP_KEY 和 BAIDU_SECRET_KEY")
		fmt.Fprintln(os.Stderr, "用法: BAIDU_APP_KEY=xxx BAIDU_SECRET_KEY=yyy go run examples/auth/main.go")
		os.Exit(1)
	}

	c := api.NewClient()
	ctx := context.Background()

	// 1. 获取设备码
	dc, err := c.Auth.DeviceCode(ctx, appKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取设备码失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("请在浏览器打开以下链接进行授权:")
	fmt.Printf("  %s?code=%s\n\n", dc.VerificationURL, dc.UserCode)
	if dc.QrcodeURL != "" {
		fmt.Printf("或扫描二维码: %s\n\n", dc.QrcodeURL)
	}

	// 2. 轮询等待用户授权
	interval := time.Duration(dc.Interval) * time.Second
	if interval < 3*time.Second {
		interval = 3 * time.Second
	}
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)

	fmt.Println("等待授权中...")
	for time.Now().Before(deadline) {
		time.Sleep(interval)

		token, err := c.Auth.DeviceToken(ctx, appKey, secretKey, dc.DeviceCode)
		if err != nil {
			// authorization_pending 是正常的等待状态，继续轮询
			if api.IsErrno(err, -1) {
				fmt.Print(".")
				continue
			}
			fmt.Fprintf(os.Stderr, "\n获取 token 失败: %v\n", err)
			os.Exit(1)
		}

		// 3. 授权成功，打印 token
		fmt.Println("\n\n授权成功!")
		fmt.Printf("Access Token:  %s\n", token.AccessToken)
		fmt.Printf("Refresh Token: %s\n", token.RefreshToken)
		fmt.Printf("Expires In:    %d 秒\n\n", token.ExpiresIn)
		fmt.Println("使用方式:")
		fmt.Printf("  export BAIDU_ACCESS_TOKEN=%s\n", token.AccessToken)
		fmt.Println("  go run examples/search/main.go 关键字")
		return
	}

	fmt.Fprintln(os.Stderr, "\n设备码已过期，请重新运行")
	os.Exit(1)
}
