package scene

import (
	"context"
	"fmt"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// UserInfo 用户基本信息（业务友好版）。
type UserInfo struct {
	// UK 用户 ID。
	UK int64

	// BaiduName 百度账号。
	BaiduName string

	// NetdiskName 网盘账号。
	NetdiskName string

	// AvatarURL 头像地址。
	AvatarURL string

	// VipType 会员类型（0=普通用户，1=普通会员，2=超级会员）。
	VipType int
}

// UserInfo 获取当前用户基本信息。
//
// 内部固定传 vip_version=v2 以获取准确会员身份。
//
// 示例:
//
//	info, err := sc.UserInfo(ctx)
func (s *Scene) UserInfo(ctx context.Context) (*UserInfo, error) {
	resp, err := s.client.Nas.UInfo(ctx, &api.UInfoParams{
		VipVersion: api.Ptr("v2"),
	})
	if err != nil {
		return nil, fmt.Errorf("scene: user info: %w", err)
	}

	return &UserInfo{
		UK:          resp.UK,
		BaiduName:   resp.BaiduName,
		NetdiskName: resp.NetdiskName,
		AvatarURL:   resp.AvatarURL,
		VipType:     resp.VipType,
	}, nil
}
