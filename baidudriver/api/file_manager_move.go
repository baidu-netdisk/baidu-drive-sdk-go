package api

import "context"

// Move 移动文件。
//
// 将一个或多个文件移动到目标目录。
//
// API 文档: https://pan.baidu.com/union/doc/mksg0s9l4
//
// 参数:
//   - async: 0=同步, 1=自适应, 2=异步
//   - filelist: 待移动的文件列表
//   - ondup: 全局重复文件处理策略（选填），可选 "fail", "newcopy", "overwrite", "skip"
func (s *FileManagerService) Move(ctx context.Context, async int, filelist []*CopyMoveItem, ondup *string) (*FileManagerResponse, error) {
	return s.doManage(ctx, "move", async, ondup, filelist)
}
