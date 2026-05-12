package api

import "context"

// Delete 删除文件。
//
// 删除一个或多个文件或目录。
//
// API 文档: https://pan.baidu.com/union/doc/mksg0s9l4
//
// 参数:
//   - async: 0=同步, 1=自适应, 2=异步
//   - paths: 待删除的文件路径列表
func (s *FileManagerService) Delete(ctx context.Context, async int, paths []string) (*FileManagerResponse, error) {
	return s.doManage(ctx, "delete", async, nil, paths)
}
