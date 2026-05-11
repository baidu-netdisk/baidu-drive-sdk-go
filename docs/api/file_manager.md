# FileManagerService — 文件管理

管理文件接口，用于对指定的文件进行复制、移动、重命名和删除操作，以及创建文件夹。

## API 方法

### Copy — 复制文件

- **API**: `POST /rest/2.0/xpan/file?method=filemanager&opera=copy`
- **文档**: https://pan.baidu.com/union/doc/mksg0s9l4

```go
resp, err := client.FileManager.Copy(ctx, 1, []*api.CopyMoveItem{
    {Path: "/test/a.txt", Dest: "/backup", Newname: "a.txt"},
}, api.Ptr("overwrite"))
```

### Move — 移动文件

- **API**: `POST /rest/2.0/xpan/file?method=filemanager&opera=move`

```go
resp, err := client.FileManager.Move(ctx, 1, []*api.CopyMoveItem{
    {Path: "/src/a.txt", Dest: "/dst", Newname: "a.txt"},
}, nil)
```

### Rename — 重命名文件

- **API**: `POST /rest/2.0/xpan/file?method=filemanager&opera=rename`

```go
resp, err := client.FileManager.Rename(ctx, 1, []*api.RenameItem{
    {Path: "/test/old.txt", Newname: "new.txt"},
})
```

### Delete — 删除文件

- **API**: `POST /rest/2.0/xpan/file?method=filemanager&opera=delete`

```go
resp, err := client.FileManager.Delete(ctx, 2, []string{"/test/a.txt"})
```

### Mkdir — 创建文件夹

- **API**: `POST /rest/2.0/xpan/file?method=create`
- **文档**: https://pan.baidu.com/union/doc/6lbaqe1lw

```go
resp, err := client.FileManager.Mkdir(ctx, &api.MkdirParams{
    Path:  "/apps/appName/mydir",
    Rtype: api.Ptr(1),
})
```

## 参数说明

### async 参数

| 值 | 说明 |
|----|------|
| 0 | 同步 |
| 1 | 自适应（服务端根据文件数目自动选择） |
| 2 | 异步 |

### ondup 参数（Copy/Move 可用）

| 值 | 说明 |
|----|------|
| `"fail"` | 返回失败（默认） |
| `"newcopy"` | 重命名文件 |
| `"overwrite"` | 覆盖 |
| `"skip"` | 跳过 |

## 响应类型

### FileManagerResponse

| 字段 | 类型 | 说明 |
|------|------|------|
| Errno | int | 错误码，0 表示成功 |
| Info | []*FileManagerInfo | 每个文件的操作结果 |
| TaskID | int64 | 异步任务 ID（async=2 时返回） |
| RequestID | uint64 | 请求唯一标识 |

### MkdirResponse

| 字段 | 类型 | 说明 |
|------|------|------|
| Errno | int | 错误码，0 表示成功 |
| FsID | int64 | 文件在云端的唯一标识 |
| Category | int | 分类类型（文件夹=6） |
| Path | string | 文件夹绝对路径 |
| Ctime | int64 | 创建时间 |
| Mtime | int64 | 修改时间 |
| Isdir | int | 是否目录（1=是） |

## 相关错误码

| 错误码 | 常量 | 说明 |
|--------|------|------|
| -7 | ErrnoFileNameIllegal | 文件名非法 |
| -8 | ErrnoFileAlreadyExist | 文件或目录已存在 |
| -9 | ErrnoPathNotExist | 文件不存在 |
| -10 | ErrnoSpaceFull | 云端容量已满 |
| 111 | ErrnoAsyncTaskRunning | 有其他异步任务正在执行 |
