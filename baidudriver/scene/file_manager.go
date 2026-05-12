package scene

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// ErrFileAlreadyExist 表示目标文件名已存在。
var ErrFileAlreadyExist = errors.New("scene: file already exists")

// ErrFileNotExist 表示源文件不存在。
var ErrFileNotExist = errors.New("scene: file does not exist")

// CopyFile 复制文件。目标已存在时返回 ErrFileAlreadyExist。
//
// src: 源文件路径
// destDir: 目标目录路径
// newname: 目标文件名
func (s *Scene) CopyFile(ctx context.Context, src, destDir, newname string) error {
	return s.CopyFileWithOndup(ctx, src, destDir, newname, "fail")
}

// CopyFileWithOndup 复制文件，支持自定义重复文件处理策略。
//
// src: 源文件路径
// destDir: 目标目录路径
// newname: 目标文件名
// ondup: 重复文件处理策略，可选值:
//   - "fail"     目标已存在时返回 ErrFileAlreadyExist（默认）
//   - "overwrite" 覆盖目标文件
//   - "newcopy"  自动重命名
//   - "skip"     跳过，不报错
func (s *Scene) CopyFileWithOndup(ctx context.Context, src, destDir, newname, ondup string) error {
	srcExists, err := s.fileExists(ctx, src)
	if err != nil {
		return fmt.Errorf("scene: copy check src: %w", err)
	}
	if !srcExists {
		return fmt.Errorf("scene: copy %q: %w", src, ErrFileNotExist)
	}

	// ondup=fail 时提前检查目标是否存在，给出更清晰的错误信息
	if ondup == "" || ondup == "fail" {
		exists, err := s.fileExistsInDir(ctx, destDir, newname)
		if err != nil {
			return fmt.Errorf("scene: copy check: %w", err)
		}
		if exists {
			return fmt.Errorf("scene: copy %q to %q: %w", src, destDir+"/"+newname, ErrFileAlreadyExist)
		}
	}

	var ondupPtr *string
	if ondup != "" {
		ondupPtr = &ondup
	}

	_, err = s.client.FileManager.Copy(ctx, 0, []*api.CopyMoveItem{
		{Path: src, Dest: destDir, Newname: newname},
	}, ondupPtr)
	if err != nil {
		return fmt.Errorf("scene: copy: %w", err)
	}
	return nil
}

// MoveFile 移动文件（操作前检查目标是否已存在）。
//
// src: 源文件路径
// destDir: 目标目录路径
// newname: 目标文件名
func (s *Scene) MoveFile(ctx context.Context, src, destDir, newname string) error {
	srcExists, err := s.fileExists(ctx, src)
	if err != nil {
		return fmt.Errorf("scene: move check src: %w", err)
	}
	if !srcExists {
		return fmt.Errorf("scene: move %q: %w", src, ErrFileNotExist)
	}

	exists, err := s.fileExistsInDir(ctx, destDir, newname)
	if err != nil {
		return fmt.Errorf("scene: move check: %w", err)
	}
	if exists {
		return fmt.Errorf("scene: move %q to %q: %w", src, destDir+"/"+newname, ErrFileAlreadyExist)
	}

	_, err = s.client.FileManager.Move(ctx, 0, []*api.CopyMoveItem{
		{Path: src, Dest: destDir, Newname: newname},
	}, nil)
	if err != nil {
		return fmt.Errorf("scene: move: %w", err)
	}
	return nil
}

// RenameFile 重命名文件（操作前检查新名是否已存在）。
//
// src: 源文件路径（完整路径）
// newname: 新文件名
func (s *Scene) RenameFile(ctx context.Context, src, newname string) error {
	srcExists, err := s.fileExists(ctx, src)
	if err != nil {
		return fmt.Errorf("scene: rename check src: %w", err)
	}
	if !srcExists {
		return fmt.Errorf("scene: rename %q: %w", src, ErrFileNotExist)
	}

	dir := path.Dir(src)
	exists, err := s.fileExistsInDir(ctx, dir, newname)
	if err != nil {
		return fmt.Errorf("scene: rename check: %w", err)
	}
	if exists {
		return fmt.Errorf("scene: rename %q to %q: %w", src, newname, ErrFileAlreadyExist)
	}

	_, err = s.client.FileManager.Rename(ctx, 0, []*api.RenameItem{
		{Path: src, Newname: newname},
	})
	if err != nil {
		return fmt.Errorf("scene: rename: %w", err)
	}
	return nil
}

// MkdirIfNotExist 创建文件夹（如果不存在）。
//
// dirPath: 文件夹绝对路径
func (s *Scene) MkdirIfNotExist(ctx context.Context, dirPath string) error {
	parentDir := path.Dir(dirPath)
	dirName := path.Base(dirPath)

	exists, err := s.fileExistsInDir(ctx, parentDir, dirName)
	if err != nil {
		return fmt.Errorf("scene: mkdir check: %w", err)
	}
	if exists {
		return nil // 已存在，直接返回
	}

	_, err = s.client.FileManager.Mkdir(ctx, &api.MkdirParams{
		Path: dirPath,
	})
	if err != nil {
		return fmt.Errorf("scene: mkdir: %w", err)
	}
	return nil
}

// FileInfo 是目录列表返回的单个文件信息（业务简化版）。
type FileInfo struct {
	FsID     int64
	Filename string
	Path     string
	IsDir    bool
	Size     int64
	Category int
	Ctime    int64
	Mtime    int64
	MD5      string
}

// ListDirOptions 是 ListDir 的可选参数。
type ListDirOptions struct {
	// Order 排序字段: "name", "time", "size"。
	Order string

	// Desc 是否降序，true=降序，false=升序。
	Desc bool

	// Start 起始位置（分页偏移量）。
	Start int

	// Limit 返回数量，0 使用服务端默认值（1000）。
	Limit int

	// FolderOnly 是否仅返回文件夹。
	FolderOnly bool
}

// ListDir 获取指定目录下的文件列表。
//
// dir: 目录路径，空字符串默认为 "/"
// opts: 可选参数，传 nil 使用默认值
func (s *Scene) ListDir(ctx context.Context, dir string, opts *ListDirOptions) ([]*FileInfo, error) {
	if dir == "" {
		dir = "/"
	}

	params := &api.ListParams{Dir: dir}
	if opts != nil {
		if opts.Order != "" {
			params.Order = &opts.Order
		}
		if opts.Desc {
			desc := 1
			params.Desc = &desc
		}
		if opts.Start > 0 {
			params.Start = &opts.Start
		}
		if opts.Limit > 0 {
			params.Limit = &opts.Limit
		}
		if opts.FolderOnly {
			folder := 1
			params.Folder = &folder
		}
	}

	resp, err := s.client.File.List(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("scene: list: %w", err)
	}

	files := make([]*FileInfo, 0, len(resp.List))
	for _, f := range resp.List {
		files = append(files, &FileInfo{
			FsID:     f.FsID,
			Filename: f.ServerFilename,
			Path:     f.Path,
			IsDir:    f.Isdir == 1,
			Size:     f.Size,
			Category: f.Category,
			Ctime:    f.ServerCtime,
			Mtime:    f.ServerMtime,
			MD5:      f.MD5,
		})
	}
	return files, nil
}

// DeleteFile 删除一个或多个文件/目录。
//
// paths: 待删除的文件路径列表
func (s *Scene) DeleteFile(ctx context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := s.client.FileManager.Delete(ctx, 0, paths)
	if err != nil {
		return fmt.Errorf("scene: delete: %w", err)
	}
	return nil
}

// fileExistsInDir 检查目录中是否存在指定文件名。
// 当目录不存在（errno=-9）时返回 (false, nil)，因为目录不存在意味着文件必然不存在。
func (s *Scene) fileExistsInDir(ctx context.Context, dir, filename string) (bool, error) {
	resp, err := s.client.File.List(ctx, &api.ListParams{Dir: dir})
	if err != nil {
		if api.IsErrno(err, api.ErrnoPathNotExist) {
			return false, nil
		}
		return false, err
	}
	for _, f := range resp.List {
		if f.ServerFilename == filename {
			return true, nil
		}
	}
	return false, nil
}

// fileExists 检查指定路径的文件是否存在。
// 通过列出父目录并匹配文件名实现。
func (s *Scene) fileExists(ctx context.Context, filePath string) (bool, error) {
	dir := path.Dir(filePath)
	filename := path.Base(filePath)
	return s.fileExistsInDir(ctx, dir, filename)
}
