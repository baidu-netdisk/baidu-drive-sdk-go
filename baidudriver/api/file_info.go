package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// FileInfo 是“获取文件信息”菜单中列表类接口返回的文件条目。
// 不同接口只会填充其中一部分字段；未下发的可选字段保持零值。
type FileInfo struct {
	FsID           int64             `json:"fs_id"`
	Path           string            `json:"path"`
	ServerFilename string            `json:"server_filename"`
	Filename       string            `json:"filename"`
	Size           int64             `json:"size"`
	ServerMtime    int64             `json:"server_mtime"`
	ServerCtime    int64             `json:"server_ctime"`
	LocalMtime     int64             `json:"local_mtime"`
	LocalCtime     int64             `json:"local_ctime"`
	Isdir          int               `json:"isdir"`
	Category       int               `json:"category"`
	MD5            string            `json:"md5"`
	Privacy        int               `json:"privacy,omitempty"`
	Unlist         int               `json:"unlist,omitempty"`
	ServerAtime    int64             `json:"server_atime,omitempty"`
	DirEmpty       *int              `json:"dir_empty,omitempty"`
	Empty          int               `json:"empty,omitempty"`
	Thumbs         map[string]string `json:"thumbs,omitempty"`
	LODocPreview   string            `json:"lodocpreview,omitempty"`
	DocPreview     string            `json:"docpreview,omitempty"`
	Dlink          string            `json:"dlink,omitempty"`
	Height         int64             `json:"height,omitempty"`
	Width          int64             `json:"width,omitempty"`
	DateTaken      int64             `json:"date_taken,omitempty"`
	Orientation    string            `json:"orientation,omitempty"`
	MediaInfo      map[string]any    `json:"media_info,omitempty"`
	ObjectKey      string            `json:"object_key,omitempty"`
	OperID         int64             `json:"oper_id,omitempty"`
	Share          int               `json:"share,omitempty"`
	ExtentTinyint1 int               `json:"extent_tinyint1,omitempty"`
}

// FileListInfoResponse 是 doclist、imagelist、videolist、btlist 的响应。
type FileListInfoResponse struct {
	Errno     int         `json:"errno"`
	Errmsg    string      `json:"errmsg,omitempty"`
	GuidInfo  string      `json:"guid_info,omitempty"`
	Guid      int64       `json:"guid,omitempty"`
	RequestID json.Number `json:"request_id"`
	Info      []*FileInfo `json:"info"`
}

// ListAllParams 是递归获取文件列表的请求参数。
type ListAllParams struct {
	Path      string
	Recursion *int
	Order     *string
	Desc      *int
	Start     *int
	Limit     *int
	Ctime     *int64
	Mtime     *int64
	Web       *int
	DeviceID  string
}

// ListAllResponse 是 listall 接口的响应。
type ListAllResponse struct {
	Errno     int         `json:"errno"`
	ShowMsg   string      `json:"show_msg,omitempty"`
	Errmsg    string      `json:"errmsg,omitempty"`
	HasMore   int         `json:"has_more"`
	Cursor    int         `json:"cursor"`
	RequestID json.Number `json:"request_id"`
	List      []*FileInfo `json:"list"`
}

// MediaListParams 是四类专用文件列表接口的公共请求参数。
// DocumentListParams、ImageListParams、VideoListParams 和 BTListParams 均为此类型别名。
type MediaListParams struct {
	ParentPath string
	Page       *int
	Num        *int
	Order      *string
	Desc       *int
	Recursion  *int
	Web        *int
}

// DocumentListParams 是获取文档列表的请求参数。
type DocumentListParams = MediaListParams

// ImageListParams 是获取图片列表的请求参数。
type ImageListParams = MediaListParams

// VideoListParams 是获取视频列表的请求参数。
type VideoListParams = MediaListParams

// BTListParams 是获取 BT 列表的请求参数。
type BTListParams = MediaListParams

// CategoryCountParams 是分类文件总个数请求参数。
type CategoryCountParams struct {
	Category   int
	ParentPath string
	Recursion  *int
}

// CategoryCountInfo 是单个分类的数量摘要。
type CategoryCountInfo struct {
	Total            int64       `json:"total"`
	Size             int64       `json:"size"`
	Count            int64       `json:"count"`
	RealServerMtime2 json.Number `json:"real_server_mtime_2"`
}

// CategoryCountResponse 是 categoryinfo 接口的响应。
type CategoryCountResponse struct {
	Errno     int                          `json:"errno"`
	Errmsg    string                       `json:"errmsg,omitempty"`
	RequestID json.Number                  `json:"request_id"`
	Info      map[string]CategoryCountInfo `json:"info"`
}

// CategoryListParams 是分类文件列表请求参数。
type CategoryListParams struct {
	Category   string
	ShowDir    *int
	ParentPath string
	Recursion  *int
	Ext        string
	Start      *int
	Limit      *int
	Order      *string
	Desc       *int
	DeviceID   string
}

// CategoryListResponse 是 categorylist 接口的响应。
type CategoryListResponse struct {
	Errno     int         `json:"errno"`
	Errmsg    string      `json:"errmsg,omitempty"`
	RequestID json.Number `json:"request_id"`
	Cursor    int         `json:"cursor"`
	HasMore   int         `json:"has_more"`
	List      []*FileInfo `json:"list"`
}

// QueryFileInfoParams 是查询文件信息请求参数。
type QueryFileInfoParams struct {
	FsIDs     []int64
	Dlink     *int
	Path      string
	Thumb     *int
	Extra     *int
	NeedMedia *int
	Detail    *int
	DeviceID  string
	FromAPaas *int
}

// QueryFileInfoResponse 是 filemetas 接口的响应。
type QueryFileInfoResponse struct {
	Errno     int            `json:"errno"`
	Errmsg    string         `json:"errmsg,omitempty"`
	RequestID json.Number    `json:"request_id"`
	Names     map[string]any `json:"names,omitempty"`
	List      []*FileInfo    `json:"list"`
}

// KeywordSearchParams 是关键词搜索请求参数。
type KeywordSearchParams struct {
	Key       string
	Dir       string
	Category  *int
	Recursion *int
	Web       *int
	DeviceID  string
}

// KeywordSearchResponse 是关键词搜索接口的响应。
type KeywordSearchResponse struct {
	Errno     int         `json:"errno"`
	Errmsg    string      `json:"errmsg,omitempty"`
	RequestID json.Number `json:"request_id"`
	HasMore   int         `json:"has_more"`
	List      []*FileInfo `json:"list"`
}

func validateBinaryFlag(name string, value *int) error {
	if value != nil && *value != 0 && *value != 1 {
		return fmt.Errorf("baidupan: %s must be 0 or 1", name)
	}
	return nil
}

func validateOrder(order *string) error {
	if order == nil {
		return nil
	}
	switch *order {
	case "name", "time", "size":
		return nil
	default:
		return fmt.Errorf("baidupan: order must be name, time, or size")
	}
}

func validatePageNum(page, num *int) error {
	if page != nil && *page < 1 {
		return fmt.Errorf("baidupan: page must be at least 1")
	}
	if num != nil && (*num < 1 || *num > 1000) {
		return fmt.Errorf("baidupan: num must be between 1 and 1000")
	}
	return nil
}

func validateStartLimit(start, limit *int) error {
	if start != nil && *start < 0 {
		return fmt.Errorf("baidupan: start must not be negative")
	}
	if limit != nil && (*limit < 1 || *limit > 1000) {
		return fmt.Errorf("baidupan: limit must be between 1 and 1000")
	}
	return nil
}

func validateCategory(category int) error {
	if category < 1 || category > 7 {
		return fmt.Errorf("baidupan: category must be between 1 and 7")
	}
	return nil
}

func setOptionalInt(q url.Values, name string, value *int) {
	if value != nil {
		q.Set(name, strconv.Itoa(*value))
	}
}

func (s *FileService) listInfo(ctx context.Context, method string, params *MediaListParams) (*FileListInfoResponse, error) {
	if params == nil {
		params = &MediaListParams{}
	}
	if err := validatePageNum(params.Page, params.Num); err != nil {
		return nil, err
	}
	if err := validateOrder(params.Order); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("desc", params.Desc); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("recursion", params.Recursion); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("web", params.Web); err != nil {
		return nil, err
	}
	q := url.Values{"method": []string{method}}
	if params.ParentPath != "" {
		q.Set("parent_path", params.ParentPath)
	}
	setOptionalInt(q, "page", params.Page)
	setOptionalInt(q, "num", params.Num)
	setOptionalString(q, "order", params.Order)
	setOptionalInt(q, "desc", params.Desc)
	setOptionalInt(q, "recursion", params.Recursion)
	setOptionalInt(q, "web", params.Web)
	var resp FileListInfoResponse
	if _, err := s.client.doGet(ctx, "/rest/2.0/xpan/file", q, &resp); err != nil {
		return nil, fmt.Errorf("file %s: %w", method, err)
	}
	return &resp, nil
}

func setOptionalString(q url.Values, name string, value *string) {
	if value != nil {
		q.Set(name, *value)
	}
}

// DocumentList 获取指定目录下的文档列表。
func (s *FileService) DocumentList(ctx context.Context, params *DocumentListParams) (*FileListInfoResponse, error) {
	return s.listInfo(ctx, "doclist", (*MediaListParams)(params))
}

// DocList 是 DocumentList 的兼容性短名称。
func (s *FileService) DocList(ctx context.Context, params *DocumentListParams) (*FileListInfoResponse, error) {
	return s.DocumentList(ctx, params)
}

// ImageList 获取指定目录下的图片列表。
func (s *FileService) ImageList(ctx context.Context, params *ImageListParams) (*FileListInfoResponse, error) {
	return s.listInfo(ctx, "imagelist", (*MediaListParams)(params))
}

// VideoList 获取指定目录下的视频列表。
func (s *FileService) VideoList(ctx context.Context, params *VideoListParams) (*FileListInfoResponse, error) {
	return s.listInfo(ctx, "videolist", (*MediaListParams)(params))
}

// BTList 获取指定路径下的 BT 文件列表。
func (s *FileService) BTList(ctx context.Context, params *BTListParams) (*FileListInfoResponse, error) {
	if params == nil {
		params = &BTListParams{}
	}
	// btlist 参数表未列 web，但官方请求示例包含 web=1；保留该参数以兼容示例。
	return s.listInfo(ctx, "btlist", (*MediaListParams)(params))
}

// BtList 是 BTList 的兼容性短名称。
func (s *FileService) BtList(ctx context.Context, params *BTListParams) (*FileListInfoResponse, error) {
	return s.BTList(ctx, params)
}

// ListAll 递归获取指定目录下的文件列表。
func (s *FileService) ListAll(ctx context.Context, params *ListAllParams) (*ListAllResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: ListAll params must not be nil")
	}
	if params.Path == "" || !strings.HasPrefix(params.Path, "/") {
		return nil, fmt.Errorf("baidupan: ListAll path must be an absolute path")
	}
	if err := validateBinaryFlag("recursion", params.Recursion); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("desc", params.Desc); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("web", params.Web); err != nil {
		return nil, err
	}
	if err := validateOrder(params.Order); err != nil {
		return nil, err
	}
	if err := validateStartLimit(params.Start, params.Limit); err != nil {
		return nil, err
	}
	if params.Ctime != nil && *params.Ctime < 0 || params.Mtime != nil && *params.Mtime < 0 {
		return nil, fmt.Errorf("baidupan: ctime and mtime must not be negative")
	}
	q := url.Values{"method": []string{"listall"}, "path": []string{params.Path}}
	setOptionalInt(q, "recursion", params.Recursion)
	setOptionalString(q, "order", params.Order)
	setOptionalInt(q, "desc", params.Desc)
	setOptionalInt(q, "start", params.Start)
	setOptionalInt(q, "limit", params.Limit)
	if params.Ctime != nil {
		q.Set("ctime", strconv.FormatInt(*params.Ctime, 10))
	}
	if params.Mtime != nil {
		q.Set("mtime", strconv.FormatInt(*params.Mtime, 10))
	}
	setOptionalInt(q, "web", params.Web)
	if params.DeviceID != "" {
		q.Set("device_id", params.DeviceID)
	}
	var resp ListAllResponse
	if _, err := s.client.doGet(ctx, "/rest/2.0/xpan/multimedia", q, &resp); err != nil {
		return nil, fmt.Errorf("file listall: %w", err)
	}
	return &resp, nil
}

// CategoryCount 获取指定目录下指定类型的文件数量。
func (s *FileService) CategoryCount(ctx context.Context, params *CategoryCountParams) (*CategoryCountResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: CategoryCount params must not be nil")
	}
	if err := validateCategory(params.Category); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("recursion", params.Recursion); err != nil {
		return nil, err
	}
	q := url.Values{"category": []string{strconv.Itoa(params.Category)}}
	if params.ParentPath != "" {
		q.Set("parent_path", params.ParentPath)
	}
	setOptionalInt(q, "recursion", params.Recursion)
	var resp CategoryCountResponse
	if _, err := s.client.doGet(ctx, "/api/categoryinfo", q, &resp); err != nil {
		return nil, fmt.Errorf("category count: %w", err)
	}
	return &resp, nil
}

// CategoryList 获取指定目录下指定类型的文件列表。
func (s *FileService) CategoryList(ctx context.Context, params *CategoryListParams) (*CategoryListResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: CategoryList params must not be nil")
	}
	if strings.TrimSpace(params.Category) == "" {
		return nil, fmt.Errorf("baidupan: CategoryList category must not be empty")
	}
	for _, raw := range strings.Split(params.Category, ",") {
		category, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("baidupan: invalid category %q", raw)
		}
		if err := validateCategory(category); err != nil {
			return nil, err
		}
	}
	if err := validateBinaryFlag("show_dir", params.ShowDir); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("recursion", params.Recursion); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("desc", params.Desc); err != nil {
		return nil, err
	}
	if err := validateOrder(params.Order); err != nil {
		return nil, err
	}
	if err := validateStartLimit(params.Start, params.Limit); err != nil {
		return nil, err
	}
	if params.Recursion != nil && *params.Recursion == 1 && params.ShowDir != nil && *params.ShowDir == 1 {
		return nil, fmt.Errorf("baidupan: show_dir=1 is not supported with recursion=1")
	}
	q := url.Values{"method": []string{"categorylist"}, "category": []string{params.Category}}
	setOptionalInt(q, "show_dir", params.ShowDir)
	if params.ParentPath != "" {
		q.Set("parent_path", params.ParentPath)
	}
	setOptionalInt(q, "recursion", params.Recursion)
	if params.Ext != "" {
		q.Set("ext", params.Ext)
	}
	setOptionalInt(q, "start", params.Start)
	setOptionalInt(q, "limit", params.Limit)
	setOptionalString(q, "order", params.Order)
	setOptionalInt(q, "desc", params.Desc)
	if params.DeviceID != "" {
		q.Set("device_id", params.DeviceID)
	}
	var resp CategoryListResponse
	if _, err := s.client.doGet(ctx, "/rest/2.0/xpan/multimedia", q, &resp); err != nil {
		return nil, fmt.Errorf("category list: %w", err)
	}
	return &resp, nil
}

// QueryFileInfo 查询一个或多个文件的 meta 信息。
func (s *FileService) QueryFileInfo(ctx context.Context, params *QueryFileInfoParams) (*QueryFileInfoResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: QueryFileInfo params must not be nil")
	}
	if len(params.FsIDs) == 0 {
		return nil, fmt.Errorf("baidupan: QueryFileInfo fsids must not be empty")
	}
	if len(params.FsIDs) > 100 {
		return nil, fmt.Errorf("baidupan: QueryFileInfo supports at most 100 fsids")
	}
	for _, id := range params.FsIDs {
		if id <= 0 {
			return nil, fmt.Errorf("baidupan: QueryFileInfo fsid must be positive")
		}
	}
	for name, value := range map[string]*int{"dlink": params.Dlink, "thumb": params.Thumb, "extra": params.Extra, "needmedia": params.NeedMedia, "detail": params.Detail, "from_apaas": params.FromAPaas} {
		if err := validateBinaryFlag(name, value); err != nil {
			return nil, err
		}
	}
	fsids, err := json.Marshal(params.FsIDs)
	if err != nil {
		return nil, fmt.Errorf("baidupan: marshal fsids: %w", err)
	}
	q := url.Values{"method": []string{"filemetas"}, "fsids": []string{string(fsids)}}
	setOptionalInt(q, "dlink", params.Dlink)
	if params.Path != "" {
		q.Set("path", params.Path)
	}
	setOptionalInt(q, "thumb", params.Thumb)
	setOptionalInt(q, "extra", params.Extra)
	setOptionalInt(q, "needmedia", params.NeedMedia)
	setOptionalInt(q, "detail", params.Detail)
	if params.DeviceID != "" {
		q.Set("device_id", params.DeviceID)
	}
	setOptionalInt(q, "from_apaas", params.FromAPaas)
	var resp QueryFileInfoResponse
	if _, err := s.client.doGet(ctx, "/rest/2.0/xpan/multimedia", q, &resp); err != nil {
		return nil, fmt.Errorf("query file info: %w", err)
	}
	return &resp, nil
}

// KeywordSearch 按文件名关键词搜索文件。接口固定使用 num=500，服务端文档不允许修改该值。
func (s *FileService) KeywordSearch(ctx context.Context, params *KeywordSearchParams) (*KeywordSearchResponse, error) {
	if params == nil {
		return nil, fmt.Errorf("baidupan: KeywordSearch params must not be nil")
	}
	if strings.TrimSpace(params.Key) == "" {
		return nil, fmt.Errorf("baidupan: KeywordSearch key must not be empty")
	}
	if utf8.RuneCountInString(params.Key) > 30 {
		return nil, fmt.Errorf("baidupan: KeywordSearch key must be at most 30 UTF-8 characters")
	}
	if err := validateBinaryFlag("recursion", params.Recursion); err != nil {
		return nil, err
	}
	if err := validateBinaryFlag("web", params.Web); err != nil {
		return nil, err
	}
	if params.Category != nil {
		if err := validateCategory(*params.Category); err != nil {
			return nil, err
		}
	}
	q := url.Values{"method": []string{"search"}, "key": []string{params.Key}, "num": []string{"500"}}
	if params.Dir != "" {
		q.Set("dir", params.Dir)
	}
	setOptionalInt(q, "category", params.Category)
	setOptionalInt(q, "recursion", params.Recursion)
	setOptionalInt(q, "web", params.Web)
	if params.DeviceID != "" {
		q.Set("device_id", params.DeviceID)
	}
	var resp KeywordSearchResponse
	if _, err := s.client.doGet(ctx, "/rest/2.0/xpan/file", q, &resp); err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}
	return &resp, nil
}
