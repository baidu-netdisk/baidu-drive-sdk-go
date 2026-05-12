package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// =============================================================================
// UniSearch 语义搜索接口
// 文档: https://pan.baidu.com/union/doc/1mgk93xgm
// =============================================================================

// UniSearchDir 是 UniSearch 接口的目录参数对象。
//
// 每个目录必须携带用户 UK 和路径。
type UniSearchDir struct {
	// UK 用户 ID。
	UK int64 `json:"uk"`

	// Path 目录路径。
	Path string `json:"path"`
}

// UniSearchParams 是 UniSearch 接口的请求参数。
//
// 所有参数通过 URL query string 传递，body 传空对象 {}。
//
// 文档: https://pan.baidu.com/union/doc/1mgk93xgm
type UniSearchParams struct {
	// Query 搜索 query（必填）。
	Query string

	// Scene 搜索场景，固定传 "mcpserver"（必填）。
	Scene string

	// Dirs 指定路径搜索（选填），每个目录包含 uk 和 path。
	Dirs []UniSearchDir

	// Category 文件类型过滤（选填）。
	// 1-视频、2-音频、3-图片、4-文档、5-应用、6-其他、7-种子
	Category []int

	// Num 搜索返回的最大数量（选填），默认 500。
	Num *int

	// Stream 是否流式响应（选填），0-否，1-是，默认 0。
	Stream *int

	// SearchType 搜索方式（选填）。
	// 0-简单搜索、1-语义搜索、2-自动，默认 0。
	SearchType *int

	// Sources 搜索来源（选填）。
	Sources []int
}

// UniSearchFileInfo 是 UniSearch 返回的单个文件信息。
type UniSearchFileInfo struct {
	// Category 文件类型。
	Category int `json:"category"`

	// Filename 文件名。
	Filename string `json:"filename"`

	// FsID 文件 ID。
	FsID int64 `json:"fsid"`

	// Isdir 是否为文件夹（0-否，1-是）。
	Isdir int `json:"isdir"`

	// Path 文件完整目录。
	Path string `json:"path"`

	// ParentPath 父目录。
	ParentPath string `json:"parent_path"`

	// Content 语义向量召回的文本段落。
	Content string `json:"content,omitempty"`

	// OCR 图片 OCR 原始文本。
	OCR string `json:"ocr,omitempty"`

	// ServerCtime 文件创建时间（Unix 时间戳）。
	ServerCtime int64 `json:"server_ctime"`

	// ServerMtime 文件修改时间（Unix 时间戳）。
	ServerMtime int64 `json:"server_mtime"`

	// Size 文件大小（byte）。
	Size int64 `json:"size"`
}

// UniSearchDataGroup 是 UniSearch 返回的搜索结果分组。
//
// API 返回的 data 字段是按 source 分组的数组，每个分组包含一个 list 数组。
type UniSearchDataGroup struct {
	// Source 搜索来源标识。
	Source int `json:"source"`

	// List 该分组下的文件列表。
	List []*UniSearchFileInfo `json:"list"`
}

// UniSearchResponse 是 UniSearch 接口的响应。
//
// 注意: API 返回的 data 是嵌套结构 data[].list[]，按 source 分组。
// request_id 在 API 中返回数字类型，使用 json.Number 兼容。
type UniSearchResponse struct {
	// Data 搜索结果分组列表（按 source 分组，每组包含 list 文件数组）。
	Data []*UniSearchDataGroup `json:"data"`

	// ErrorMsg 错误信息。
	ErrorMsg string `json:"error_msg"`

	// ErrorNo 错误码，0 表示成功。
	ErrorNo int `json:"error_no"`

	// ExtraInfo 额外信息。
	ExtraInfo map[string]any `json:"extra_info,omitempty"`

	// IsEnd 是否结束。
	IsEnd bool `json:"is_end"`

	// RequestID 请求 ID（API 返回数字类型，使用 json.Number 兼容）。
	RequestID json.Number `json:"request_id"`

	// ServerTime 服务器时间。
	ServerTime int64 `json:"server_time"`
}

// Files 返回所有分组中的文件列表（扁平化）。
//
// 便捷方法，将 data[].list[] 嵌套结构展平为单一文件列表。
func (r *UniSearchResponse) Files() []*UniSearchFileInfo {
	var files []*UniSearchFileInfo
	for _, group := range r.Data {
		if group == nil {
			continue
		}
		for _, f := range group.List {
			if f != nil {
				files = append(files, f)
			}
		}
	}
	return files
}

// UniSearch 调用 /xpan/unisearch 语义搜索接口。
//
// 接口地址: POST https://pan.baidu.com/xpan/unisearch
// 参数通过 URL query string 传递，body 传空对象 {}。
//
// 文档: https://pan.baidu.com/union/doc/1mgk93xgm
//
// 请求示例:
//
//	resp, err := client.File.UniSearch(ctx, &api.UniSearchParams{
//	    Query: "关键词",
//	    Scene: "mcpserver",
//	    Dirs:  []api.UniSearchDir{{UK: 123, Path: "/test"}},
//	    Num:   api.Ptr(100),
//	})
//	// 获取扁平化文件列表
//	files := resp.Files()
func (s *FileService) UniSearch(ctx context.Context, params *UniSearchParams) (*UniSearchResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: UniSearch params must not be nil")
	}
	if params.Query == "" {
		return nil, fmt.Errorf("baidupan: UniSearch query must not be empty")
	}
	if params.Scene == "" {
		params.Scene = "mcpserver"
	}

	// 参数放在 URL query string 中
	q := url.Values{}
	q.Set("query", params.Query)
	q.Set("scene", params.Scene)
	// 服务端要求 dirs 参数使用 JSON 对象数组格式（如 [{"uk":123,"path":"/test"}]）。
	if len(params.Dirs) > 0 {
		dirsJSON, _ := json.Marshal(params.Dirs)
		q.Set("dirs", string(dirsJSON))
	}
	if len(params.Category) > 0 {
		catJSON, _ := json.Marshal(params.Category)
		q.Set("category", string(catJSON))
	}
	if params.Num != nil {
		q.Set("num", strconv.Itoa(*params.Num))
	}
	if params.Stream != nil {
		q.Set("stream", strconv.Itoa(*params.Stream))
	}
	if params.SearchType != nil {
		q.Set("search_type", strconv.Itoa(*params.SearchType))
	}
	if len(params.Sources) > 0 {
		srcJSON, _ := json.Marshal(params.Sources)
		q.Set("sources", string(srcJSON))
	}

	// body 传空对象 {}（API 有非空校验）
	emptyBody := struct{}{}
	var resp UniSearchResponse
	_, err := s.client.doPostJSON(ctx, "/xpan/unisearch", q, emptyBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("unisearch: %w", err)
	}
	return &resp, nil
}
