# Baidu Netdisk Go SDK

[![Build Status](https://github.com/baidu-netdisk/baidu-drive-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/baidu-netdisk/baidu-drive-sdk-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/baidu-netdisk/baidu-drive-sdk-go.svg)](https://pkg.go.dev/github.com/baidu-netdisk/baidu-drive-sdk-go)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

百度网盘 Go SDK，对齐[百度网盘开放平台](https://pan.baidu.com/union/doc/zksg0sb9z)最新 API。

## 安装

```bash
go get github.com/baidu-netdisk/baidu-drive-sdk-go
```

**要求**: Go 1.24+

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
    "github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene"
)

func main() {
    client := api.NewClient(
        api.WithAccessToken("YOUR_ACCESS_TOKEN"),
    )
    sc := scene.New(client)

    ctx := context.Background()

    // 获取用户信息
    user, err := sc.UserInfo(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Hello, %s!\n", user.BaiduName)

    // 列出根目录文件
    files, err := sc.ListDir(ctx, "/", &scene.ListDirOptions{Limit: 20})
    if err != nil {
        log.Fatal(err)
    }
    for _, f := range files {
        fmt.Printf("  %s (%d bytes)\n", f.Filename, f.Size)
    }
}
```

## 架构

SDK 分为两层：

- **scene 层** (`baidudriver/scene`) — **推荐使用**。业务场景封装，提供自动重试、多接口串联、前置检查等高层能力
- **api 层** (`baidudriver/api`) — 底层 API 调用，与百度网盘开放平台接口一一对应，适合需要精细控制的场景

```
scene（业务层，推荐）
  └── api（接口层）
        └── HTTP
```

## Scene 业务层（推荐）

scene 层基于 api 层封装，提供业务级便捷方法，包含自动重试（exponential backoff）、前置检查、多接口串联等能力。**大多数场景下推荐直接使用 scene 层。**

```go
client := api.NewClient(api.WithAccessToken("YOUR_ACCESS_TOKEN"))
sc := scene.New(client)
```

### 用户信息

```go
// 用户基本信息
info, _ := sc.UserInfo(ctx)
fmt.Println(info.UK, info.BaiduName, info.VipType)

// 用户信息 + IoT 会员权限（整合查询，一次调用）
nasInfo, _ := sc.NasUserInfo(ctx, "your_device_id")
fmt.Println(nasInfo.BaiduName, nasInfo.HasPrivilege, nasInfo.IsSVIP, nasInfo.IsIoTSVIP)
```

### 文件浏览与搜索

```go
// 目录列表
files, _ := sc.ListDir(ctx, "/", &scene.ListDirOptions{
    Order: "time", Desc: true, Limit: 10,
})

// 语义搜索（自动获取 UK，支持按目录、文件类型过滤）
results, _ := sc.Search(ctx, &scene.SearchParams{
    Query:    "合同",
    Dir:      "/documents",
    Category: []int{4}, // 4=文档
})
```

### 上传与下载

```go
// 一键上传：自动完成 Precreate → 分片上传 → CreateFile，带重试
result, _ := sc.UploadFile(ctx, &scene.UploadFileParams{
    LocalPath:  "/local/file.zip",
    RemotePath: "/apps/myapp/file.zip",
    RType:      api.Ptr(3), // 冲突时自动重命名
})

// 一键下载：自动获取 dlink + 流式下载到本地，带重试
result, _ := sc.DownloadFile(ctx, &scene.DownloadFileParams{
    FsID:      123456789,
    LocalPath: "/tmp/downloaded.txt",
})
```

### 文件管理

文件管理方法在操作前自动检查源文件和目标文件是否存在，避免无效请求：

```go
// 复制
sc.CopyFile(ctx, "/src.txt", "/backup", "src_copy.txt")

// 移动
sc.MoveFile(ctx, "/src.txt", "/archive", "src.txt")

// 重命名
sc.RenameFile(ctx, "/old.txt", "new.txt")

// 删除
sc.DeleteFile(ctx, []string{"/trash.txt"})

// 创建文件夹（已存在则跳过）
sc.MkdirIfNotExist(ctx, "/new_folder")
```

### Scene 方法一览

| 方法 | 说明 |
|------|------|
| UserInfo | 获取用户基本信息 |
| NasUserInfo | 用户信息 + IoT 权限（整合查询） |
| Search | 语义搜索（自动获取 UK） |
| ListDir | 目录文件列表 |
| UploadFile | 一键上传（自动分片 + 重试） |
| DownloadFile | 一键下载（自动获取 dlink + 重试） |
| CopyFile, MoveFile, RenameFile | 文件操作（带前置检查） |
| DeleteFile | 删除文件 |
| MkdirIfNotExist | 创建文件夹（已存在则跳过） |

## 认证 (Auth)

使用 SDK 前需先通过 OAuth 获取 access_token。支持两种授权方式：

```go
client := api.NewClient()

// 1. 授权码模式
token, _ := client.Auth.Code2Token(ctx, "app_key", "secret_key", "auth_code", "oob")

// 2. 设备码模式（适用于 CLI）
device, _ := client.Auth.DeviceCode(ctx, "app_key")
// 用户扫码后...
token, _ := client.Auth.DeviceToken(ctx, "app_key", "secret_key", device.DeviceCode)

// 拿到 token 后创建带认证的客户端
client = api.NewClient(api.WithAccessToken(token.AccessToken))
```

## 错误处理

```go
result, err := client.File.List(ctx, params)
if err != nil {
    // 检查特定错误码
    if api.IsErrno(err, api.ErrnoAccessDenied) {
        // Token 过期或权限不足
    }

    // 获取完整错误信息
    var apiErr *api.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("errno=%d msg=%s\n", apiErr.Errno, apiErr.Errmsg)
        fmt.Printf("url=%s body=%s\n", apiErr.URL, apiErr.ResponseBody)
    }
}
```

常用错误码常量：

| 常量 | 值 | 含义 |
|------|------|------|
| `ErrnoAccessDenied` | -6 | Token 过期或权限不足 |
| `ErrnoFileNameIllegal` | -7 | 文件名不合法 |
| `ErrnoFileAlreadyExist` | -8 | 文件已存在 |
| `ErrnoPathNotExist` | -9 | 路径不存在 |
| `ErrnoSpaceFull` | -10 | 空间已满 |
| `ErrnoParamError` | 2 | 参数错误 |
| `ErrnoLimitExceeded` | 31034 | 频率超限 |

## 客户端选项

```go
client := api.NewClient(
    api.WithAccessToken("token"),       // Access Token
    api.WithHTTPClient(customClient),   // 自定义 HTTP Client
    api.WithBaseURL("https://..."),     // 自定义 API 地址
    api.WithPCSBaseURL("https://..."),  // 自定义 PCS 上传地址
    api.WithDebug(true),                // 调试模式（打印请求/响应）
    api.WithLogger(os.Stderr),          // 自定义调试日志输出
)
```

## Context 超时控制

所有 API 方法均接受 `context.Context` 参数，可通过 `context.WithTimeout` 控制单次请求超时：

```go
import (
    "context"
    "time"
)

// 单次请求 10 秒超时
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

files, err := sc.ListDir(ctx, "/", nil)
if err != nil {
    // 超时会返回 context.DeadlineExceeded
    log.Fatal(err)
}
```

对于大文件上传/下载等耗时操作，建议设置更长的超时或使用 `context.WithCancel` 手动控制：

```go
// 大文件上传使用较长超时
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
defer cancel()

result, err := sc.UploadFile(ctx, &scene.UploadFileParams{
    LocalPath:  "/local/large_file.zip",
    RemotePath: "/apps/myapp/large_file.zip",
})
```

## API 底层接口

以下为 api 层的底层接口，适合需要精细控制请求参数的高级用户。一般场景推荐使用上方的 scene 层。

### 用户信息 (Nas)

```go
// 用户基本信息
user, _ := client.Nas.UInfo(ctx, &api.UInfoParams{
    VipVersion: api.Ptr("v2"),
})

// 存储容量
quota, _ := client.Nas.Quota(ctx, nil)

// IoT 设备用户权限查询
iot, _ := client.Nas.IoTQueryUInfo(ctx, &api.IoTQueryUInfoParams{
    DeviceID: "your_device_id",
})
```

### 文件操作 (File)

```go
// 列出文件
files, _ := client.File.List(ctx, &api.ListParams{Dir: "/", Limit: api.Ptr(20)})

// 语义搜索
results, _ := client.File.UniSearch(ctx, &api.UniSearchParams{
    Query: "report", Scene: "mcpserver", Category: []int{4},
})
allFiles := results.Files()
```

### 文件管理 (FileManager)

```go
// 复制（async: 0=同步, 1=自适应, 2=异步）
client.FileManager.Copy(ctx, 0, []*api.CopyMoveItem{
    {Path: "/a.txt", Dest: "/backup", Newname: "a_copy.txt"},
}, nil)

// 移动
client.FileManager.Move(ctx, 0, []*api.CopyMoveItem{
    {Path: "/a.txt", Dest: "/archive", Newname: "a.txt"},
}, nil)

// 重命名
client.FileManager.Rename(ctx, 0, []*api.RenameItem{
    {Path: "/old.txt", Newname: "new.txt"},
})

// 删除
client.FileManager.Delete(ctx, 0, []string{"/trash.txt"})

// 创建文件夹
client.FileManager.Mkdir(ctx, &api.MkdirParams{Path: "/new_folder"})
```

### 上传 (Upload)

```go
// 三步流程：Precreate → SliceUpload → CreateFile
precreate, _ := client.Upload.Precreate(ctx, &api.PrecreateParams{
    Path: "/apps/myapp/test.txt", Size: 1024,
    BlockList: []string{"ab56b4d92b40713acc5af89985d4b786"},
})
slice, _ := client.Upload.SliceUpload(ctx, &api.SliceUploadParams{
    Path: "/apps/myapp/test.txt", UploadID: precreate.UploadID, PartSeq: 0, File: reader,
})
file, _ := client.Upload.CreateFile(ctx, &api.CreateFileParams{
    Path: "/apps/myapp/test.txt", Size: 1024, UploadID: precreate.UploadID,
    BlockList: []string{slice.MD5},
})
```

### 下载 (Download)

```go
// 两步流程：Meta（获取 dlink）→ Download（流式下载）
meta, _ := client.Download.Meta(ctx, &api.MetaParams{
    FsIDs: []int64{123456789},
})
body, size, _ := client.Download.Download(ctx, &api.DownloadParams{
    Dlink: meta.List[0].Dlink,
})
defer body.Close()
io.Copy(localFile, body)
```

### API 方法一览

| Service | 方法 | 说明 |
|---------|------|------|
| Auth | Code2Token, DeviceCode, DeviceToken | OAuth 2.0 授权 |
| Nas | UInfo, Quota, IoTQueryUInfo | 用户信息、容量、IoT 权限 |
| File | List, UniSearch | 文件列表、语义搜索 |
| FileManager | Copy, Move, Rename, Delete, Mkdir | 文件管理 |
| Upload | Precreate, SliceUpload, CreateFile | 文件上传（三步） |
| Download | Meta, Download | 文件下载（两步） |

## 示例

查看 [examples/](examples/) 目录：

- `examples/auth/` — OAuth 设备码授权流程
- `examples/nas_user_info/` — NAS 用户信息 + IoT 会员权限查询
- `examples/quota/` — 网盘容量查询
- `examples/file_list/` — 文件列表（带 limit 参数验证）
- `examples/search_pdf/` — 语义搜索（按文档类型过滤）

## CI/CD

本项目使用 GitHub Actions 进行持续集成。工作流配置位于 [.github/workflows/ci.yml](.github/workflows/ci.yml)，每次 push 或 PR 会自动运行测试并生成覆盖率报告。

## License

Apache-2.0
