# Changelog

本文件按版本记录百度网盘开放平台 Go SDK 对外提供的能力。

## 0.2.0

### 概览

一个面向服务的 Go SDK：API 层与开放平台接口一一对应，Scene 层提供常见业务编排（列目录、搜索、上传、下载）。

### 授权

- `auth.Begin(ctx, auth.Config)` 启动目录授权，返回授权 URL 与会话；`session.Wait(ctx)` 等待回调并拿到已认证的 `api.Client`。
- 目录级授权：授权页按目录授予访问范围，SDK 把完整凭证写入 macOS Keychain、Windows Credential Manager 或 Linux Secret Service，不静默落明文文件。
- 安全基线：随机 `state`、精确 loopback 回调校验、失败回调立即结束等待、回调判重；`RedirectURI` 支持 `localhost`，SDK 只监听 `127.0.0.1`。
- 有效凭证复用，过期后重新授权；SDK 不持久化应用 `SecretKey`。

### 文件与目录

- 列目录：`File.List`、`Scene.ListDir`，条目含 `fs_id`、路径、大小、分类、MD5，以及服务端时间（`Ctime`/`Mtime`）与本地时间（`LocalCtime`/`LocalMtime`）。
- 文件管理：`FileManager.Mkdir`、`Move`、`Copy`、`Rename`、`Delete`。
- 上传：`Upload.Precreate` → `SliceUpload` → `CreateFile` 三步，`Scene.UploadFile` 封装完整流程，支持保留本地修改时间。
- 下载：`Download.Meta`、`Download`，`Scene.DownloadFile` 封装两步。

### 文件信息与搜索

- 文件信息：`QueryFileInfo`、`ListAll`、`CategoryList`、`CategoryCount`、`DocumentList`/`DocList`、`ImageList`、`VideoList`、`BTList`/`BtList`。
- 搜索：`File.UniSearch`（语义/关键词检索）与 `KeywordSearch`、`Scene.Search`。
- 本地入参校验：分页、分类、排序、检索数量等非法值在 SDK 侧直接报错。

### 用户与容量

- `Nas.UInfo`、`Nas.Quota`、`Nas.IoTQueryUInfo`。

### 其他

- `APIError` 暴露原始 `errno`，并提供常用错误码常量，便于按错误码分支。
- 每次请求以查询参数注入 `access_token`；token 过期后的重新授权、限流与重试由调用方负责，API 层不自动重试。

### 升级提示

- 授权入口改为 `auth.Begin` + `session.Wait`，返回的 `api.Client` 直接用于后续调用；原来在 API 层构造客户端的方式不再适用。
