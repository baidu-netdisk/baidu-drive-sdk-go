// NAS 用户信息查询示例（整合用户基本信息 + IoT 权限信息）
//
// 使用方式:
//   1. 设置 BAIDU_APP_KEY、BAIDU_SECRET_KEY、BAIDU_REDIRECT_URI
//   2. 运行: go run ./examples/nas_user_info <device_id>

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene"
	"github.com/baidu-netdisk/baidu-drive-sdk-go/examples/internal/authclient"
)

func main() {
	if err := run(os.Getenv, os.Args); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func run(getenv func(string) string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("错误: 请提供 device_id 参数\n用法: go run main.go <device_id>")
	}
	deviceID := args[1]

	client, err := authclient.New(context.Background(), getenv, os.Stdout)
	if err != nil {
		return err
	}
	sc := scene.New(client)

	return runWithScene(sc, deviceID)
}

func runWithScene(sc *scene.Scene, deviceID string) error {
	fmt.Println("正在查询 NAS 用户信息...")
	fmt.Println()

	ctx := context.Background()
	info, err := sc.NasUserInfo(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("查询失败: %v", err)
	}

	fmt.Println("========================================")
	fmt.Println("          用户基本信息")
	fmt.Println("========================================")
	fmt.Printf("UK:       %d\n", info.UK)
	fmt.Printf("百度账号: %s\n", info.BaiduName)
	fmt.Printf("网盘账号: %s\n", info.NetdiskName)
	fmt.Printf("头像地址: %s\n", info.AvatarURL)
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("          IoT 权限信息")
	fmt.Println("========================================")
	fmt.Printf("是否有特权:    %s\n", yesNo(info.HasPrivilege))
	fmt.Printf("是否 SVIP:     %s\n", yesNo(info.IsSVIP))
	fmt.Printf("是否 IoT SVIP: %s\n", yesNo(info.IsIoTSVIP))
	if info.StartTime > 0 {
		fmt.Printf("IoT SVIP 开始: %s\n", formatTime(info.StartTime))
	}
	if info.EndTime > 0 {
		fmt.Printf("IoT SVIP 结束: %s\n", formatTime(info.EndTime))
	}
	fmt.Printf("服务器时间:    %s\n", time.Unix(info.Now, 0).Format("2006-01-02 15:04:05"))
	fmt.Println("========================================")
	return nil
}

func yesNo(v int) string {
	if v == 1 {
		return "是"
	}
	return "否"
}

func formatTime(ts uint64) string {
	return time.Unix(int64(ts), 0).Format("2006-01-02 15:04:05")
}
