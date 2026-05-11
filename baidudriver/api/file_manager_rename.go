package api

import "context"

// Rename 重命名文件。
//
// 对一个或多个文件进行重命名。
//
// API 文档: https://pan.baidu.com/union/doc/mksg0s9l4
//
// 参数:
//   - async: 0=同步, 1=自适应, 2=异步
//   - filelist: 待重命名的文件列表
func (s *FileManagerService) Rename(ctx context.Context, async int, filelist []*RenameItem) (*FileManagerResponse, error) {
	return s.doManage(ctx, "rename", async, nil, filelist)
}
