// Package scene 提供基于 api 层的业务场景封装。
//
// scene 包只调用 api 包做二次业务逻辑处理和多接口串联调用，
// 不直接进行任何 HTTP 请求。所有网络操作都委托给 api.Client。
package scene

import (
	"context"
	"fmt"
	"sync"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// Scene 基于 api.Client 提供业务级便捷方法。
//
// Scene 不直接进行 HTTP 调用，所有接口请求都委托给底层的 api.Client。
type Scene struct {
	client   *api.Client
	mu       sync.Mutex
	cachedUK int64
}

// New 创建一个 Scene 实例。
//
// 示例:
//
//	client := api.NewClient(api.WithAccessToken("your_token"))
//	sc := scene.New(client)
func New(client *api.Client) *Scene {
	return &Scene{client: client}
}

// Client 返回底层的 api.Client，用于直接调用低级 API。
func (s *Scene) Client() *api.Client {
	return s.client
}

// getUK 获取当前用户的 UK。
//
// 首次调用通过 Nas.UInfo 接口获取，后续复用缓存。
// 失败时不锁定缓存，下次调用可重试。
func (s *Scene) getUK(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cachedUK != 0 {
		return s.cachedUK, nil
	}

	resp, err := s.client.Nas.UInfo(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("scene: getUK: %w", err)
	}
	if resp.UK == 0 {
		return 0, fmt.Errorf("scene: getUK: server returned uk=0")
	}
	s.cachedUK = resp.UK
	return s.cachedUK, nil
}

// =============================================================================
// Search 业务场景封装
// =============================================================================

// SearchParams 搜索参数（最小暴露原则，仅业务需要字段）。
type SearchParams struct {
	// Query 搜索关键词（必填）。
	Query string

	// Dir 指定目录搜索（选填），默认为根目录 "/"。
	Dir string

	// Category 文件类型过滤（选填）。
	// 1-视频、2-音频、3-图片、4-文档、5-应用、6-其他、7-种子
	Category []int

	// Num 返回结果数量上限（选填），默认 500。
	Num int
}

// SearchResult 搜索结果项。
type SearchResult struct {
	// FsID 文件 ID。
	FsID int64

	// Filename 文件名。
	Filename string

	// Path 文件路径。
	Path string

	// IsDir 是否为文件夹。
	IsDir bool

	// Category 文件类型。
	Category int

	// Size 文件大小（byte）。
	Size int64

	// Ctime 创建时间。
	Ctime int64

	// Mtime 修改时间。
	Mtime int64

	// Content 语义搜索匹配内容。
	Content string
}

// Search 执行语义搜索（业务层封装，最小暴露）。
//
// 仅暴露业务需要的参数：query、dir、num。
// 内部调用 api.File.UniSearch 接口。
//
// 文档: https://pan.baidu.com/union/doc/1mgk93xgm
//
// 示例:
//
//	results, err := sc.Search(ctx, &scene.SearchParams{
//	    Query: "关键词",
//	    Dir:   "/test",
//	    Num:   100,
//	})
func (s *Scene) Search(ctx context.Context, params *SearchParams) ([]*SearchResult, error) {
	if params == nil {
		return nil, fmt.Errorf("scene: Search params must not be nil")
	}
	if params.Query == "" {
		return nil, fmt.Errorf("scene: Search query must not be empty")
	}

	apiParams := &api.UniSearchParams{
		Query: params.Query,
		Scene: "mcpserver",
	}

	// Dir 转换：自动获取 uk，构建 UniSearchDir 对象
	if params.Dir != "" {
		uk, err := s.getUK(ctx)
		if err != nil {
			return nil, fmt.Errorf("scene: search: %w", err)
		}
		apiParams.Dirs = []api.UniSearchDir{{UK: uk, Path: params.Dir}}
	}

	// Category 转换
	if len(params.Category) > 0 {
		apiParams.Category = params.Category
	}

	// Num 转换
	if params.Num > 0 {
		apiParams.Num = api.Ptr(params.Num)
	}

	resp, err := s.client.File.UniSearch(ctx, apiParams)
	if err != nil {
		return nil, fmt.Errorf("scene: search: %w", err)
	}

	// 使用 Files() 展平嵌套的 data[].list[] 结构
	files := resp.Files()
	results := make([]*SearchResult, 0, len(files))
	for _, item := range files {
		results = append(results, &SearchResult{
			FsID:     item.FsID,
			Filename: item.Filename,
			Path:     item.Path,
			IsDir:    item.Isdir == 1,
			Category: item.Category,
			Size:     item.Size,
			Ctime:    item.ServerCtime,
			Mtime:    item.ServerMtime,
			Content:  item.Content,
		})
	}

	return results, nil
}
