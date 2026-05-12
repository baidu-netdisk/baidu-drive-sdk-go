package scene

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// =============================================================================
// NasUserInfo 测试
// =============================================================================

func TestScene_NasUserInfo(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("uinfo method = %q, want GET", r.Method)
			}
			q := r.URL.Query()
			if q.Get("vip_version") != "v2" {
				t.Errorf("vip_version = %q, want v2", q.Get("vip_version"))
			}
			w.Write([]byte(`{
				"errno": 0,
				"errmsg": "succ",
				"request_id": 111,
				"baidu_name": "testuser",
				"netdisk_name": "网盘用户",
				"avatar_url": "https://example.com/avatar.jpg",
				"vip_type": 2,
				"uk": 9876543210
			}`))
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("iot method = %q, want GET", r.Method)
			}
			q := r.URL.Query()
			if q.Get("device_id") != "test_device_123" {
				t.Errorf("device_id = %q, want test_device_123", q.Get("device_id"))
			}
			w.Write([]byte(`{
				"request_id": 222,
				"error_code": 0,
				"error_msg": "succ",
				"data": {
					"has_privilege": 1,
					"is_svip": 1,
					"is_iot_svip": 0,
					"start_time": 1700000000,
					"end_time": 1730000000,
					"now": 1710000000
				}
			}`))
		},
	})
	defer ts.Close()

	info, err := sc.NasUserInfo(context.Background(), "test_device_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证 UInfo 字段
	if info.UK != 9876543210 {
		t.Errorf("UK = %d, want 9876543210", info.UK)
	}
	if info.BaiduName != "testuser" {
		t.Errorf("BaiduName = %q, want testuser", info.BaiduName)
	}
	if info.NetdiskName != "网盘用户" {
		t.Errorf("NetdiskName = %q, want 网盘用户", info.NetdiskName)
	}
	if info.AvatarURL != "https://example.com/avatar.jpg" {
		t.Errorf("AvatarURL = %q", info.AvatarURL)
	}

	// 验证 IoTQueryUInfo 字段
	if info.HasPrivilege != 1 {
		t.Errorf("HasPrivilege = %d, want 1", info.HasPrivilege)
	}
	if info.IsSVIP != 1 {
		t.Errorf("IsSVIP = %d, want 1", info.IsSVIP)
	}
	if info.IsIoTSVIP != 0 {
		t.Errorf("IsIoTSVIP = %d, want 0", info.IsIoTSVIP)
	}
	if info.StartTime != 1700000000 {
		t.Errorf("StartTime = %d, want 1700000000", info.StartTime)
	}
	if info.EndTime != 1730000000 {
		t.Errorf("EndTime = %d, want 1730000000", info.EndTime)
	}
	if info.Now != 1710000000 {
		t.Errorf("Now = %d, want 1710000000", info.Now)
	}
}

func TestScene_NasUserInfo_IoTDataNil(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{
				"errno": 0,
				"errmsg": "succ",
				"request_id": 111,
				"baidu_name": "testuser",
				"netdisk_name": "网盘用户",
				"avatar_url": "https://example.com/avatar.jpg",
				"vip_type": 0,
				"uk": 1234567890
			}`))
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			// data 字段为空，模拟无 IoT 数据的场景
			w.Write([]byte(`{
				"request_id": 222,
				"error_code": 0,
				"error_msg": "succ"
			}`))
		},
	})
	defer ts.Close()

	info, err := sc.NasUserInfo(context.Background(), "device_no_iot")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// UInfo 字段正常
	if info.UK != 1234567890 {
		t.Errorf("UK = %d, want 1234567890", info.UK)
	}
	if info.BaiduName != "testuser" {
		t.Errorf("BaiduName = %q, want testuser", info.BaiduName)
	}

	// IoT 字段全部为零值
	if info.HasPrivilege != 0 {
		t.Errorf("HasPrivilege = %d, want 0", info.HasPrivilege)
	}
	if info.IsSVIP != 0 {
		t.Errorf("IsSVIP = %d, want 0", info.IsSVIP)
	}
	if info.IsIoTSVIP != 0 {
		t.Errorf("IsIoTSVIP = %d, want 0", info.IsIoTSVIP)
	}
	if info.StartTime != 0 {
		t.Errorf("StartTime = %d, want 0", info.StartTime)
	}
	if info.EndTime != 0 {
		t.Errorf("EndTime = %d, want 0", info.EndTime)
	}
	if info.Now != 0 {
		t.Errorf("Now = %d, want 0", info.Now)
	}
}

func TestScene_NasUserInfo_UInfoError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			t.Error("iot query should not be called when uinfo fails")
		},
	})
	defer ts.Close()

	_, err := sc.NasUserInfo(context.Background(), "device_123")
	if err == nil {
		t.Fatal("expected error when uinfo fails")
	}
	if !strings.Contains(err.Error(), "uinfo") {
		t.Errorf("error should mention uinfo, got: %v", err)
	}
}

func TestScene_NasUserInfo_IoTQueryError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{
				"errno": 0,
				"errmsg": "succ",
				"request_id": 111,
				"baidu_name": "testuser",
				"netdisk_name": "网盘用户",
				"avatar_url": "https://example.com/avatar.jpg",
				"vip_type": 2,
				"uk": 9876543210
			}`))
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"request_id":333,"error_code":-1,"error_msg":"device not found"}`))
		},
	})
	defer ts.Close()

	_, err := sc.NasUserInfo(context.Background(), "bad_device")
	if err == nil {
		t.Fatal("expected error when iot query fails")
	}
	if !strings.Contains(err.Error(), "iot") {
		t.Errorf("error should mention iot, got: %v", err)
	}
}
