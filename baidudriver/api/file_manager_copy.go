package api

import "context"

// Copy 复制文件。
//
// 将一个或多个文件复制到目标目录。
//
// API 文档: https://pan.baidu.com/union/doc/mksg0s9l4
//
// 参数:
//   - async: 0=同步, 1=自适应, 2=异步
//   - filelist: 待复制的文件列表
//   - ondup: 全局重复文件处理策略（选填），可选 "fail", "newcopy", "overwrite", "skip"
//
// 示例:
//
//	resp, err := client.FileManager.Copy(ctx, 1, []*api.CopyMoveItem{
//	    {Path: "/test/a.txt", Dest: "/test/backup", Newname: "a.txt"},
//	}, api.Ptr("overwrite"))
func (s *FileManagerService) Copy(ctx context.Context, async int, filelist []*CopyMoveItem, ondup *string) (*FileManagerResponse, error) {
	return s.doManage(ctx, "copy", async, ondup, filelist)
}
