# Baidu Netdisk Go SDK

[![Build Status](https://github.com/baidu-netdisk/baidu-drive-sdk-go/actions/workflows/ci.yml/badge.svg)](https://github.com/baidu-netdisk/baidu-drive-sdk-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/baidu-netdisk/baidu-drive-sdk-go.svg)](https://pkg.go.dev/github.com/baidu-netdisk/baidu-drive-sdk-go)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

百度网盘开放平台 Go SDK，提供开发者授权、文件浏览与搜索、上传下载、文件管理、用户与容量查询、NAS 权益查询。API 层与开放平台接口一一对应，Scene 层提供业务编排。

本项目采用 [Apache-2.0](LICENSE) 许可证，公开版本变化见 [CHANGELOG.md](CHANGELOG.md)。

## 安装与兼容性

```bash
go get github.com/baidu-netdisk/baidu-drive-sdk-go
```

要求 Go 1.24 或更高版本。生产使用应固定到包含所需接口、经过验证的 commit 或正式 release。

## 快速开始

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/auth"
    "github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
    defer cancel()

    session, err := auth.Begin(ctx, auth.Config{
        AppKey:      os.Getenv("BAIDU_APP_KEY"),
        SecretKey:   os.Getenv("BAIDU_SECRET_KEY"),
        RedirectURI: os.Getenv("BAIDU_REDIRECT_URI"),
        OpenBrowser: true,
    })
    if err != nil {
        log.Fatal(err)
    }
    if session.BrowserOpenError != nil {
        fmt.Printf("自动打开浏览器失败，请在运行 SDK 的同一设备浏览器中打开：%s\n", session.AuthorizationURL)
    }
    result, err := session.Wait(ctx)
    if err != nil {
        log.Fatal(err)
    }

    sc := scene.New(result.Client)

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

SDK 由一个开发者授权入口和两层业务调用组成：

- **auth 入口** (`baidudriver/auth`) — 唯一的 directory loopback OAuth 接入，返回已认证的 `api.Client`
- **scene 层** (`baidudriver/scene`) — **推荐使用**。业务场景封装，提供有限重试、多接口串联、前置检查等高层能力
- **api 层** (`baidudriver/api`) — 底层 API 调用，与百度网盘开放平台接口一一对应，适合需要精细控制的场景

```
auth（授权入口）──→ api.Client
                       ↑
scene（业务层，推荐）──┘
  └── api（接口层）──→ HTTP
```

## Scene 业务层（推荐）

scene 层基于 api 层封装，提供业务级便捷方法，包含有限重试、前置检查、多接口串联等能力。**大多数场景下推荐直接使用 scene 层。**

```go
// result 来自下方唯一授权流程的 session.Wait(ctx)。
sc := scene.New(result.Client)
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

获取文件信息菜单的完整只读接口在 `api.File` 上提供：`ListAll`、`DocumentList`、`ImageList`、`VideoList`、`BTList`、`CategoryCount`、`CategoryList`、`QueryFileInfo`、`KeywordSearch` 和 `UniSearch`。这些接口均接受 `context.Context`，服务端错误以 `*api.APIError` 返回；`QueryFileInfo` 的 `FsIDs` 最多 100 个。

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

面向开发者的公开授权契约为 directory loopback OAuth。先在百度网盘开发者控制台登记一个精确的回调地址，例如 `http://localhost:8902/baidu/oauth`；主机支持 `localhost`、`127.0.0.1` 或 `::1`，必须包含固定端口和回调路径。`localhost` 内部固定监听 `127.0.0.1`，回调 Host 仍按登记值校验。应用使用两个调用：`auth.Begin(ctx, cfg)` 和 `session.Wait(ctx)`。

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/auth"
)

ctx := context.Background()
session, err := auth.Begin(ctx, auth.Config{
    AppKey:      os.Getenv("BAIDU_APP_KEY"),
    SecretKey:   os.Getenv("BAIDU_SECRET_KEY"),
    RedirectURI: "http://127.0.0.1:8787/oauth/callback",
    OpenBrowser: true,
})
if err != nil {
    return err
}
if session.BrowserOpenError != nil {
    fmt.Printf("自动打开浏览器失败，请在运行 SDK 的同一设备浏览器中打开：%s\n", session.AuthorizationURL)
}

result, err := session.Wait(ctx)
if err != nil {
    return err
}
client := result.Client
```

`Begin` 会先读取本机凭证，再在需要重新授权时先启动 loopback server，随后生成 H5/Web 授权链接，避免浏览器回调早于监听器启动。设置 `OpenBrowser: true` 时，SDK 会请求系统默认外部浏览器打开该链接；失败原因写入 `Session.BrowserOpenError`，`AuthorizationURL` 仍可在运行 SDK 的同一设备浏览器中打开。SDK 不内嵌 WebView，避免把账号口令、Cookie 和 SSO 放入应用自有 Web 容器。`Wait` 只消费一次回调，校验 `state`，使用回调中的短时 `code` 换取 token，原子写入凭证存储并返回已注入 access token 的 `Result.Client`。如果本机凭证仍有效，`AuthorizationURL` 为空，`Wait` 会直接返回且 `Result.FromStore` 为 `true`。

排查授权链路时可在 `auth.Config` 中设置 `Debug: true` 和 `Logger: os.Stderr`。授权日志只记录脱敏后的生命周期事件、参数名称和长度、HTTP 状态、耗时及链路标识，不记录 AppKey、SecretKey、code、state、token、完整授权 URL、callback query 或 token 请求 URL。该开关只作用于授权阶段；如需业务 API 调试日志，仍需通过 `ClientOptions` 单独配置 `api.WithDebug(true)`。

默认协议固定为：

- 授权端点：`https://pan.baidu.com/union/oauth/authorize`，参数包含 `response_type=code`、申请范围 `scope=basic,netdisk`、`authorization_type=directory` 和随机 `state`。
- 换 token 端点：`GET https://pan.baidu.com/xpan/oauth/get?method=token`，SDK 传递 `client_id`、`client_secret`、一次性 `code`、完全一致的 `redirect_uri`以及与授权请求相同的 `state`。
- 换 token 成功响应的凭证字段位于根对象的 `data` 中：`access_token`、`refresh_token`、`expires_in`、`scope`、`authorized_paths` 和 `redirect_uri`；`errno`、`request_id`、`server_time` 等状态字段位于根对象。`Credential.RequestedScope` 保存 SDK 发起授权时申请的 scope，`Credential.Scope` 保存 `data.scope` 返回的实际授权 scope，两者不能混用。
- 凭证存储：默认服务名为 `com.baidu.netdisk.openapi-go`，写入 macOS Keychain、Windows Credential Manager 或 Linux Secret Service。SDK 不会静默回退到明文文件；桌面凭证服务不可用时会返回错误。测试或短生命周期工具必须显式注入 `CredentialStore`。
- 生命周期：仍有效且对应同一申请 scope 的凭证会复用；过期或临近过期时重新授权。当前版本不自动刷新、撤销 token，也不持久化应用的 `SecretKey`。

以上默认 `code` 模式为现有调用方保留，不使用 PKCE；其换 token 请求仍使用带敏感参数的 `GET`。授权 URL、回调参数和凭证不得写入日志。

### 真实链路快速验证

`TestLiveDirectoryOAuth` 会自动请求系统浏览器打开授权页，输出脱敏授权诊断日志，并验证授权 URL、loopback 回调、code 换 token、系统密码库存储、只读 Quota API 和凭证复用。它默认跳过；在 zsh 中可安全读入变量并直接透传给测试，无需把值写入命令历史：

```zsh
read -s "BAIDU_APP_KEY?AppKey: "; echo
read -s "BAIDU_SECRET_KEY?SecretKey: "; echo
read "BAIDU_REDIRECT_URI?Redirect URI: "

BAIDU_AUTH_LIVE=1 \
  BAIDU_APP_KEY="$BAIDU_APP_KEY" \
  BAIDU_SECRET_KEY="$BAIDU_SECRET_KEY" \
  BAIDU_REDIRECT_URI="$BAIDU_REDIRECT_URI" \
  go test -v ./baidudriver/auth \
    -run '^TestLiveDirectoryOAuth$' -count=1 -timeout=10m
```

`BAIDU_REDIRECT_URI` 必须与开发者控制台登记值完全一致，使用空闲的固定 loopback 端口和非根路径。测试使用独立的 `com.baidu.netdisk.openapi-go.live-test` 密码库服务，并在测试前后删除哈希隔离的测试凭证；macOS、Windows 或 Linux 桌面环境可能弹出系统密码库权限提示。不要把应用凭证贴到聊天、提交到仓库或写入命令参数。

## API 层边界

`api` 层不自动重试。默认客户端超时为 30 秒；传入 `api.WithHTTPClient` 后完全使用调用方的 client 配置。access token 会作为查询参数注入请求，调试输出和 `APIError` 诊断会脱敏。JSON 响应最多读取 10 MiB，响应 body 会被消费并关闭。上传分片请求会整体缓冲以设置 Content-Length，代码注释建议单分片不超过 4 MiB，但底层不会校验分片大小或 PartSeq。

API 方法包括：`File.List`、`DocumentList`/`DocList`、`ImageList`、`VideoList`、`BTList`/`BtList`、`ListAll`、`CategoryCount`、`CategoryList`、`QueryFileInfo`、`KeywordSearch`、`UniSearch`；`FileManager.Copy`、`Move`、`Rename`、`Delete`、`Mkdir`；`Nas.UInfo`、`Quota`、`IoTQueryUInfo`；`Upload.Precreate`、`SliceUpload`、`CreateFile`；`Download.Meta`、`Download`。

主要本地校验：`QueryFileInfo` 的 fsids 为 1–100 个正数；`KeywordSearch` 的 key 非空且不超过 30 个 UTF-8 字符，num 固定 500；`UniSearch` 的 query 必填、scene 固定 `mcpserver`、num 为 1–500；`CategoryList` 的 category 为 1–7 且 recursion=1 时不允许 show_dir=1。其余约束由服务端返回。

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

自定义 HTTP client、API 地址或调试输出时，通过 `auth.Config.ClientOptions` 传入；不要在常规开发者接入中手工注入 token：

```go
session, err := auth.Begin(ctx, auth.Config{
    AppKey:      os.Getenv("BAIDU_APP_KEY"),
    SecretKey:   os.Getenv("BAIDU_SECRET_KEY"),
    RedirectURI: "http://127.0.0.1:8787/oauth/callback",
    OpenBrowser: true,
    ClientOptions: []api.Option{
        api.WithHTTPClient(customClient),   // 自定义 HTTP Client
        api.WithBaseURL("https://..."),     // 自定义 API 地址
        api.WithPCSBaseURL("https://..."),  // 自定义 PCS 上传地址
        api.WithDebug(true),                // 调试模式（打印请求/响应）
        api.WithLogger(os.Stderr),          // 自定义调试日志输出
    },
})
// Begin 会先启动回调 listener，再请求系统浏览器打开授权页；
// 从 Result.Client 取得应用上述选项的客户端。
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
| auth（独立包） | Begin, (*Session).Wait | 唯一的 directory loopback OAuth 开发者入口 |
| Nas | UInfo, Quota, IoTQueryUInfo | 用户信息、容量、IoT 权限 |
| File | List, ListAll, DocumentList, ImageList, VideoList, BTList, CategoryCount, CategoryList, QueryFileInfo, KeywordSearch, UniSearch | 文件列表、分类/类型列表、文件 meta 和关键词/语义搜索 |
| FileManager | Copy, Move, Rename, Delete, Mkdir | 文件管理 |
| Upload | Precreate, SliceUpload, CreateFile | 文件上传（三步） |
| Download | Meta, Download | 文件下载（两步） |

## 示例

查看 [examples/](examples/) 目录：

- `examples/auth/` — directory loopback OAuth 授权流程
- `examples/nas_user_info/` — NAS 用户信息 + IoT 会员权限查询
- `examples/quota/` — 网盘容量查询
- `examples/file_list/` — 文件列表（带 limit 参数验证）
- `examples/search_pdf/` — 语义搜索（按文档类型过滤）

## CI/CD

本项目使用 GitHub Actions 进行持续集成。工作流配置位于 [.github/workflows/ci.yml](.github/workflows/ci.yml)，每次 push 或 PR 会自动运行测试并生成覆盖率报告。

## License

Apache-2.0
