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
