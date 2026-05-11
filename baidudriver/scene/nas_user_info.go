package scene

import (
	"context"
	"fmt"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

// NasUserInfo 用户基本信息 + IoT 权限信息（业务友好版）。
type NasUserInfo struct {
	// UK 用户 ID。
	UK int64

	// BaiduName 百度账号。
	BaiduName string

	// NetdiskName 网盘账号。
	NetdiskName string

	// AvatarURL 头像地址。
	AvatarURL string

	// HasPrivilege 是否有特权（0:无, 1:有），当用户是SVIP或IoT SVIP未过期时为1。
	HasPrivilege int

	// IsSVIP 是否是SVIP（0:否, 1:是）。
	IsSVIP int

	// IsIoTSVIP 是否是IoT SVIP（0:否, 1:是）。
	IsIoTSVIP int

	// StartTime IoT SVIP开始时间（Unix时间戳）。
	StartTime uint64

	// EndTime IoT SVIP结束时间（Unix时间戳）。
	EndTime uint64

	// Now 当前服务器时间（Unix时间戳）。
	Now int64
}

// NasUserInfo 获取当前用户基本信息及 IoT 权限信息。
//
// 内部调用 UInfo（固定 vip_version=v2）和 IoTQueryUInfo 两个接口，
// 将用户基本信息（不含 VipType）与 IoT 权限信息整合为一个结构返回。
//
// 示例:
//
//	info, err := sc.NasUserInfo(ctx, "your_device_id")
func (s *Scene) NasUserInfo(ctx context.Context, deviceID string) (*NasUserInfo, error) {
	uinfoResp, err := s.client.Nas.UInfo(ctx, &api.UInfoParams{
		VipVersion: api.Ptr("v2"),
	})
	if err != nil {
		return nil, fmt.Errorf("scene: nas user info: uinfo: %w", err)
	}

	iotResp, err := s.client.Nas.IoTQueryUInfo(ctx, &api.IoTQueryUInfoParams{
		DeviceID: deviceID,
	})
	if err != nil {
		return nil, fmt.Errorf("scene: nas user info: iot query uinfo: %w", err)
	}

	result := &NasUserInfo{
		UK:          uinfoResp.UK,
		BaiduName:   uinfoResp.BaiduName,
		NetdiskName: uinfoResp.NetdiskName,
		AvatarURL:   uinfoResp.AvatarURL,
	}

	if iotResp.Data != nil {
		result.HasPrivilege = iotResp.Data.HasPrivilege
		result.IsSVIP = iotResp.Data.IsSVIP
		result.IsIoTSVIP = iotResp.Data.IsIoTSVIP
		result.StartTime = iotResp.Data.StartTime
		result.EndTime = iotResp.Data.EndTime
		result.Now = iotResp.Data.Now
	}

	return result, nil
}
