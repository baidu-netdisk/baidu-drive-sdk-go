package api

// UploadService handles file upload operations.
//
// 百度网盘上传流程分为三步:
//  1. Precreate — 预创建文件，获取 uploadid
//  2. SliceUpload — 分片上传文件数据
//  3. Create — 合并分片，创建最终文件
//
// 文档: https://pan.baidu.com/union/doc/3ksg0s9ye
type UploadService service
