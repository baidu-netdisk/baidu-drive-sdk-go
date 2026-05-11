package api

// DownloadService handles file download operations.
//
// 百度网盘下载流程分为两步:
//  1. Meta — 获取文件元信息（含 dlink 下载链接）
//  2. Download — 通过 dlink 流式下载文件内容
//
// 文档: https://pan.baidu.com/union/doc/pkuo3snyp
type DownloadService service
