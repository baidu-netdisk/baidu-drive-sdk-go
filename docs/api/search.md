# Search API — api 层接口文档

> 包路径: `github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api`
>
> 官方文档: https://pan.baidu.com/union/doc/zksg0sb9z

## 概述

api 包是对百度网盘开放平台 HTTP 接口的 1:1 封装，请求参数和响应字段完全对齐官方文档。

**与 scene 层的关系**:

```
调用方（你的业务）
    ↓
scene 层 — 业务语义封装（分页、类型转换、默认值）
    ↓
api 层   — 1:1 对齐官方 HTTP 接口（本文档）
    ↓
百度网盘开放平台 REST API
```

- **api 层**: 原始接口，参数/响应与官方文档一一对应，适合需要完全控制请求的场景
- **scene 层**: 业务封装，提供更简洁的类型和便捷方法，推荐日常使用（见 [scene 层文档](../scene/search.md)）

## 初始化

开发者通过唯一的 directory loopback OAuth 获取客户端。先在开发者控制台登记与 `RedirectURI` 完全一致的 loopback 地址，然后只调用 `auth.Begin` 和 `Session.Wait`：

```go
import (
    "fmt"
    "os"

    "github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
    "github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/auth"
)

session, err := auth.Begin(ctx, auth.Config{
    AppKey:      os.Getenv("BAIDU_APP_KEY"),
    SecretKey:   os.Getenv("BAIDU_SECRET_KEY"),
    RedirectURI: "http://127.0.0.1:8787/oauth/callback",
    OpenBrowser: true,
    ClientOptions: []api.Option{
        api.WithDebug(true),
    },
})
if err != nil {
    return err
}

// SDK 会在 listener 就绪后请求系统浏览器打开授权页；失败时保留 URL。
if session.BrowserOpenError != nil {
    fmt.Println(session.AuthorizationURL)
}

result, err := session.Wait(ctx)
if err != nil {
    return err
}
client := result.Client
```

默认授权端点为 `https://pan.baidu.com/union/oauth/authorize`，换 token 端点为 `GET https://pan.baidu.com/xpan/oauth/get?method=token`；换 token 时会再次提交同一个 `state`，成功响应的凭证字段位于根对象的 `data` 中。`auth` 包负责外部浏览器拉起、PKCE S256、`state`、loopback 回调、防重放、token 交换和系统钥匙串持久化，并分别保存申请 scope 与 `data.scope` 返回的实际授权 scope。它当前不自动刷新 token，过期后应重新执行上述两个调用。百度换 token 仍使用敏感查询参数的 `GET`，且服务端是否强制 PKCE 未核实，所以该流程是 OAuth 2.1-ready 基线而非完整合规声明。

### Client Options

| Option | 说明 |
|--------|------|
| `WithAccessToken(token)` | 底层 token 注入；`auth.Result.Client` 内部使用，常规开发者接入不要用它替代授权流程 |
| `WithBaseURL(url)` | 自定义 base URL（默认 `https://pan.baidu.com`） |
| `WithHTTPClient(hc)` | 自定义 `*http.Client` |
| `WithDebug(true)` | 开启请求/响应调试日志 |
| `WithLogger(w)` | 自定义日志 Writer |

需要这些选项时通过 `auth.Config.ClientOptions` 传入，由 `Session.Wait` 构造最终客户端。默认凭证存储是 macOS Keychain、Windows Credential Manager 或 Linux Secret Service；SDK 不会静默回退到明文文件。

---

## FileService.Search

搜索用户网盘中的文件和目录。

```go
func (s *FileService) Search(ctx context.Context, params *SearchParams) (*SearchResponse, error)
```

**请求方式**: `GET https://pan.baidu.com/rest/2.0/xpan/file?method=search`

### 参数 `SearchParams`

| 字段 | 类型 | 必填 | 官方参数名 | 说明 |
|------|------|------|-----------|------|
| `Key` | `string` | 是 | `key` | 搜索关键字，上限 30 字符（UTF8） |
| `Dir` | `*string` | 否 | `dir` | 搜索目录，默认根目录 |
| `Category` | `*int` | 否 | `category` | 文件类型筛选（1-7） |
| `Num` | `*int` | 否 | `num` | 返回条数，默认 500 |
| `Recursion` | `*int` | 否 | `recursion` | 1=递归搜索子目录 |
| `Web` | `*int` | 否 | `web` | 1=返回缩略图 `thumbs` 字段 |
| `DeviceID` | `*string` | 否 | `device_id` | 设备 ID（硬件设备必传） |
| `Page` | `*int` | 否 | `page` | 页码 |

> 可选参数使用指针类型，配合 `api.Ptr()` 辅助函数设置：
> ```go
> params := &api.SearchParams{
>     Key:       "照片",
>     Recursion: api.Ptr(1),
>     Web:       api.Ptr(1),
>     Num:       api.Ptr(20),
> }
> ```

### 响应 `SearchResponse`

| 字段 | 类型 | JSON | 说明 |
|------|------|------|------|
| `Errno` | `int` | `errno` | 错误码，0 表示成功 |
| `RequestID` | `int64` | `request_id` | 请求唯一标识 |
| `HasMore` | `int` | `has_more` | 0=无更多，1=有更多 |
| `DisplayCount` | `int` | `display_count` | 展示数量 |
| `NeedAISearch` | `bool` | `need_ai_search` | 是否需要 AI 搜索 |
| `List` | `[]*SearchFileInfo` | `list` | 搜索结果文件列表 |
| `ContentList` | `[]SearchFileInfo` | `contentlist` | 内容列表 |

### SearchFileInfo

| 字段 | 类型 | JSON | 说明 |
|------|------|------|------|
| `FsID` | `int64` | `fs_id` | 文件唯一标识 |
| `Path` | `string` | `path` | 文件完整路径 |
| `ServerFilename` | `string` | `server_filename` | 文件名 |
| `Size` | `int64` | `size` | 文件大小（字节） |
| `Category` | `int` | `category` | 文件类型（1-7） |
| `Isdir` | `int` | `isdir` | 0=文件，1=目录 |
| `LocalCtime` | `int64` | `local_ctime` | 客户端创建时间 |
| `LocalMtime` | `int64` | `local_mtime` | 客户端修改时间 |
| `ServerCtime` | `int64` | `server_ctime` | 服务端创建时间 |
| `ServerMtime` | `int64` | `server_mtime` | 服务端修改时间 |
| `MD5` | `string` | `md5` | 云端哈希值 |
| `Score` | `float64` | `score` | 搜索相关性得分 |
| `RelevanceLevel` | `int` | `relevance_level` | 相关性等级 |
| `DocPreview` | `string` | `docpreview` | 文档预览内容 |
| `Fold` | `int` | `fold` | 折叠标识 |
| `Thumbs` | `*SearchThumbs` | `thumbs` | 缩略图（需 `Web=1`） |

### SearchThumbs

| 字段 | 类型 | JSON | 说明 |
|------|------|------|------|
| `URL1` | `string` | `url1` | 小缩略图（~140x90） |
| `URL2` | `string` | `url2` | 中缩略图（~360x270） |
| `URL3` | `string` | `url3` | 大缩略图（~850x580） |
| `Icon` | `string` | `icon` | 图标（~60x60） |

### 错误处理

| 条件 | 错误 |
|------|------|
| `params == nil` | `baidupan: Search params must not be nil` |
| `params.Key == ""` | `baidupan: Search key must not be empty` |
| API 返回非零 errno | `*api.APIError` |

```go
resp, err := client.File.Search(ctx, params)
if err != nil {
    if api.IsErrno(err, -6) {
        // access denied
    }
    log.Fatal(err)
}
```

### 示例

```go
resp, err := client.File.Search(ctx, &api.SearchParams{
    Key:       "mmexport",
    Dir:       api.Ptr("/测试目录"),
    Recursion: api.Ptr(1),
    Web:       api.Ptr(1),
})
if err != nil {
    log.Fatal(err)
}
for _, f := range resp.List {
    fmt.Printf("[%d] %s (%d bytes)\n", f.FsID, f.ServerFilename, f.Size)
    if f.Thumbs != nil {
        fmt.Printf("    缩略图: %s\n", f.Thumbs.URL2)
    }
}
```

---

## Category 常量

| 常量 | 值 | 说明 |
|------|----|------|
| `CategoryVideo` | 1 | 视频 |
| `CategoryAudio` | 2 | 音频 |
| `CategoryImage` | 3 | 图片 |
| `CategoryDoc` | 4 | 文档 |
| `CategoryApp` | 5 | 应用 |
| `CategoryOther` | 6 | 其他 |
| `CategoryTorrent` | 7 | 种子 |

---

## 辅助函数

### Ptr

将值转为指针，用于设置可选参数。

```go
func Ptr[T any](v T) *T
```

```go
api.Ptr(1)        // *int
api.Ptr("path")   // *string
```

### IsErrno

判断错误是否为特定 errno。

```go
func IsErrno(err error, errno int) bool
```

---

## 错误码参考

完整错误码列表见官方文档: https://pan.baidu.com/union/doc/okumlx17r

常见错误码:

| errno | 说明 |
|-------|------|
| 0 | 成功 |
| -6 | 身份验证失败 |
| -7 | 文件或目录不存在 |
| -9 | 文件不存在 |
| 111 | access_token 无效 |
