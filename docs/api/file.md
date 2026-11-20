# File 接口文档

## UniSearch 语义搜索

文档: https://pan.baidu.com/union/doc/1mgk93xgm

### 方法

```go
func (s *FileService) UniSearch(ctx context.Context, params *UniSearchParams) (*UniSearchResponse, error)
```

### 请求参数

```go
type UniSearchDir struct {
    UK   int64  `json:"uk"`   // 用户 ID
    Path string `json:"path"` // 目录路径
}

type UniSearchParams struct {
    Query      string           // 搜索关键词（必填）
    Scene      string           // 搜索场景，固定传 "mcpserver"（必填）
    Dirs       []UniSearchDir   // 指定路径搜索（选填），每个目录包含 uk 和 path
    Category   []int            // 文件类型过滤（选填）: 1-视频、2-音频、3-图片、4-文档、5-应用、6-其他、7-种子
    Num        *int             // 返回数量上限（选填），默认 500
    Stream     *int             // 是否流式响应（选填），0-否、1-是
    SearchType *int             // 搜索方式（选填），0-简单搜索、1-语义搜索、2-自动
    Sources    []int            // 搜索来源（选填）
}
```

### 响应结构

```go
type UniSearchResponse struct {
    Data       []*UniSearchFileInfo // 搜索结果列表
    ErrorMsg   string               // 错误信息
    ErrorNo    int                  // 错误码，0 表示成功
    ExtraInfo  map[string]any       // 额外信息
    IsEnd      bool                 // 是否结束
    RequestID  string               // 请求 ID
    ServerTime int64                // 服务器时间
}

type UniSearchFileInfo struct {
    Category    int    // 文件类型
    Filename    string // 文件名
    FsID        int64  // 文件 ID
    Isdir       int    // 是否为文件夹（0-否，1-是）
    Path        string // 文件完整路径
    ParentPath  string // 父目录
    Content     string // 语义向量召回的文本段落
    OCR         string // 图片 OCR 原始文本
    ServerCtime int64  // 创建时间（Unix 时间戳）
    ServerMtime int64  // 修改时间（Unix 时间戳）
    Size        int64  // 文件大小（byte）
    PID         int64  // 语义搜索文本段落 ID（有内容召回时返回）
}
```

### 示例

```go
// 先获取 uk
uinfo, err := client.Nas.UInfo(ctx, nil)
if err != nil {
    log.Fatal(err)
}

resp, err := client.File.UniSearch(ctx, &api.UniSearchParams{
    Query: "工作文档",
    Scene: "mcpserver",
    Dirs:  []api.UniSearchDir{{UK: uinfo.UK, Path: "/工作"}},
    Num:   api.Ptr(50),
})
if err != nil {
    log.Fatal(err)
}

for _, file := range resp.Data {
    fmt.Printf("%s (%d bytes)\n", file.Filename, file.Size)
    if file.Content != "" {
        fmt.Printf("  匹配内容: %s\n", file.Content)
    }
}
```

### 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| -6 | access denied |
| -7 | file name illegal |
| -9 | path not exist |

该接口使用 `error_no`/`error_msg` 错误格式。

## 获取文件信息菜单

以下接口对应百度网盘开放平台“基础网盘服务 -> 获取文件信息”菜单，均使用 OAuth `access_token` 查询参数。开发者应通过 `auth.Begin(ctx, auth.Config)` 和 `session.Wait(ctx)` 取得 `Result.Client`；`auth` 包负责 directory loopback、token 交换、系统钥匙串持久化和有效凭证复用。路径参数必须使用以 `/` 开头的绝对路径；`FileService` 本身不重新授权或刷新 token。

| 方法 | 官方 method/path | 说明 |
| --- | --- | --- |
| `List` | `GET /rest/2.0/xpan/file?method=list` | 当前目录文件列表（已有接口） |
| `ListAll` | `GET /rest/2.0/xpan/multimedia?method=listall` | 递归获取文件列表 |
| `DocumentList` | `GET /rest/2.0/xpan/file?method=doclist` | 文档列表 |
| `ImageList` | `GET /rest/2.0/xpan/file?method=imagelist` | 图片列表 |
| `VideoList` | `GET /rest/2.0/xpan/file?method=videolist` | 视频列表 |
| `BTList` | `GET /rest/2.0/xpan/file?method=btlist` | BT 列表 |
| `CategoryCount` | `GET /api/categoryinfo` | 指定类型的文件数量 |
| `CategoryList` | `GET /rest/2.0/xpan/multimedia?method=categorylist` | 指定类型的文件列表 |
| `QueryFileInfo` | `GET /rest/2.0/xpan/multimedia?method=filemetas` | 一个或多个文件的 meta |
| `KeywordSearch` | `GET /rest/2.0/xpan/file?method=search` | 文件名关键词搜索 |
| `UniSearch` | `POST /xpan/unisearch` | 简单或语义搜索 |

### 通用类型和示例

四个类型列表方法共用 `MediaListParams`（`DocumentListParams`、`ImageListParams`、`VideoListParams`、`BTListParams` 是语义化别名）：

```go
resp, err := client.File.ImageList(ctx, &api.ImageListParams{
    ParentPath: "/apps/myapp",
    Page: api.Ptr(1), Num: api.Ptr(100),
    Order: api.Ptr("time"), Desc: api.Ptr(1), Web: api.Ptr(1),
})
```

`ListAll` 使用 `Start`/`Limit` 分页；当 `HasMore == 1` 时以响应的 `Cursor` 作为下一次查询起点。该接口仅支持当前应用授权目录 `/apps/{appname}`，且官方文档建议请求频率不超过每分钟 8-10 次；实际测试时应使用已在当前应用授权的目录。

```go
resp, err := client.File.ListAll(ctx, &api.ListAllParams{
    Path: "/apps/myapp", Recursion: api.Ptr(1),
    Start: api.Ptr(0), Limit: api.Ptr(1000),
})
```

分类、meta 和关键词搜索示例：

```go
count, _ := client.File.CategoryCount(ctx, &api.CategoryCountParams{
    Category: 4, ParentPath: "/apps/myapp",
})
files, _ := client.File.CategoryList(ctx, &api.CategoryListParams{
    Category: "3,4", ParentPath: "/apps/myapp", Limit: api.Ptr(100),
})
meta, _ := client.File.QueryFileInfo(ctx, &api.QueryFileInfoParams{
    FsIDs: []int64{123456789}, Dlink: api.Ptr(1), Extra: api.Ptr(1),
})
hits, _ := client.File.KeywordSearch(ctx, &api.KeywordSearchParams{
    Key: "README", Dir: "/apps/myapp",
})
```

`QueryFileInfo` 按文档在 SDK 本地限制最多 100 个 `FsIDs`，即使服务端对重复 ID 的数量可能更宽松。`CategoryList` 在 `Recursion=1` 时不支持 `ShowDir=1`。`BTList` 的官方参数表未列 `web`，但官方示例出现该参数，SDK 保留 `Web` 字段以兼容示例。

### 响应和边界

列表接口返回 `FileInfo`，覆盖路径、文件名、fs_id、大小、时间、类型、md5、缩略图、下载链接、图片尺寸、视频 `media_info` 等字段。空结果是成功响应，服务端非零 `errno` 会作为 `*api.APIError` 返回。SDK 的 `auth` 包负责凭证持久化、到期判断和已认证客户端构造，但当前不自动刷新或撤销 token；凭证过期时调用方需重新执行两个调用的授权流程。调用方仍负责控制台回调配置、应用密钥保护、权限 scope、频控和破坏性操作审批；`FileService` 负责参数校验、请求编码、响应解码和 context 取消。
