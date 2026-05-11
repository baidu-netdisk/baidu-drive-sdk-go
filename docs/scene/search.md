# Search 业务场景文档

> 包路径: `github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene`
>
> 底层 API 文档: https://pan.baidu.com/union/doc/1mgk93xgm

## 概述

Scene 层的 `Search` 方法是对 API 层 `UniSearch` 的业务封装，遵循**最小暴露原则**，仅暴露业务需要的参数。

`Search` 会自动调用 `UInfo` 接口获取用户 UK 并缓存，用于构建 `dirs` 参数，用户无需手动传入。

```go
sc := scene.New(client)
results, err := sc.Search(ctx, &scene.SearchParams{
    Query: "关键词",
    Dir:   "/文档",
    Num:   50,
})
```

## Scene.Search

执行语义搜索（业务层封装，最小暴露）。

```go
func (s *Scene) Search(ctx context.Context, params *SearchParams) ([]*SearchResult, error)
```

### 请求参数

```go
type SearchParams struct {
    Query string // 搜索关键词（必填）
    Dir   string // 指定目录搜索（选填），默认根目录 "/"
    Num   int    // 返回结果数量上限（选填），默认 500
}
```

**字段说明**:

| 字段 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `Query` | `string` | 是 | — | 搜索关键词 |
| `Dir` | `string` | 否 | `""` (全盘) | 搜索目录 |
| `Num` | `int` | 否 | `500` | 返回结果数量上限 |

### 响应结构

```go
type SearchResult struct {
    FsID     int64  // 文件 ID
    Filename string // 文件名
    Path     string // 文件路径
    IsDir    bool   // 是否为文件夹
    Category int    // 文件类型
    Size     int64  // 文件大小（byte）
    Ctime    int64  // 创建时间（Unix 时间戳）
    Mtime    int64  // 修改时间（Unix 时间戳）
    Content  string // 语义搜索匹配内容
}
```

### 错误

| 条件 | 错误信息 |
|------|----------|
| `params == nil` | `scene: Search params must not be nil` |
| `params.Query == ""` | `scene: Search query must not be empty` |
| API 返回非零 error_no | `*api.APIError`（可用 `api.IsErrno` 判断） |

### 示例

```go
results, err := sc.Search(ctx, &scene.SearchParams{
    Query: "财务报表",
    Dir:   "/工作文档",
    Num:   20,
})
if err != nil {
    log.Fatal(err)
}

for _, r := range results {
    if r.IsDir {
        fmt.Printf("[目录] %s/\n", r.Filename)
    } else {
        fmt.Printf("[文件] %s (%d bytes)\n", r.Filename, r.Size)
    }
    if r.Content != "" {
        fmt.Printf("  匹配: %s\n", r.Content)
    }
}
```

---

## 与 API 层的区别

| 特性 | Scene.Search | api.UniSearch |
|------|--------------|---------------|
| 暴露参数 | Query, Dir, Category, Num | 全部参数 |
| Dir 类型 | string | []UniSearchDir |
| UK 获取 | 自动（UInfo 缓存） | 需手动传入 |
| IsDir 类型 | bool | int (0/1) |
| 时间字段 | Ctime, Mtime | ServerCtime, ServerMtime |
| 内部固定 | Scene="mcpserver" | 需手动指定 |
| 语义内容 | Content | Content |

---

## 架构关系

```
scene.Scene
  └── Search(params)
        ├── getUK(ctx)  ──→ api.NasService.UInfo (缓存 uk)
        └── api.FileService.UniSearch(params)
                └── POST /xpan/unisearch
```

scene 层不直接发起 HTTP 请求，所有网络调用委托给 `api.Client`。

---

## Category 常量

定义在 `api` 包中，用于文件类型判断。

| 常量 | 值 | 说明 |
|------|----|------|
| `api.CategoryVideo` | 1 | 视频 |
| `api.CategoryAudio` | 2 | 音频 |
| `api.CategoryImage` | 3 | 图片 |
| `api.CategoryDoc` | 4 | 文档 |
| `api.CategoryApp` | 5 | 应用 |
| `api.CategoryOther` | 6 | 其他 |
| `api.CategoryTorrent` | 7 | 种子 |
