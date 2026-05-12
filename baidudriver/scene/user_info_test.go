package scene

import (
	"context"
	"net/http"
	"testing"
)

// =============================================================================
// UserInfo 测试
// =============================================================================

func TestScene_UserInfo(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("method = %q, want GET", r.Method)
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
	})
	defer ts.Close()

	info, err := sc.UserInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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
	if info.VipType != 2 {
		t.Errorf("VipType = %d, want 2", info.VipType)
	}
}

func TestScene_UserInfo_APIError(t *testing.T) {
	ts, sc := newRouterScene(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
		},
	})
	defer ts.Close()

	_, err := sc.UserInfo(context.Background())
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}
