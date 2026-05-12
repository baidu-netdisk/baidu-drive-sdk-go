package api

import (
	"net/http"
	"testing"
)

// =============================================================================
// Nas.UInfo 测试
// =============================================================================

func TestNas_UInfo(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		q := r.URL.Query()
		if q.Get("method") != "uinfo" {
			t.Errorf("method param = %q, want uinfo", q.Get("method"))
		}

		w.Write([]byte(`{
			"errno": 0,
			"errmsg": "succ",
			"request_id": 1234567890,
			"baidu_name": "testuser",
			"netdisk_name": "网盘用户",
			"avatar_url": "https://example.com/avatar.jpg",
			"vip_type": 2,
			"uk": 9876543210
		}`))
	})
	defer ts.Close()

	resp, err := c.Nas.UInfo(ctx(), &UInfoParams{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Errno != 0 {
		t.Errorf("Errno = %d, want 0", resp.Errno)
	}
	if resp.BaiduName != "testuser" {
		t.Errorf("BaiduName = %q, want testuser", resp.BaiduName)
	}
	if resp.NetdiskName != "网盘用户" {
		t.Errorf("NetdiskName = %q, want 网盘用户", resp.NetdiskName)
	}
	if resp.AvatarURL != "https://example.com/avatar.jpg" {
		t.Errorf("AvatarURL = %q", resp.AvatarURL)
	}
	if resp.VipType != 2 {
		t.Errorf("VipType = %d, want 2", resp.VipType)
	}
	if resp.UK != 9876543210 {
		t.Errorf("UK = %d, want 9876543210", resp.UK)
	}
	if resp.RequestID.String() != "1234567890" {
		t.Errorf("RequestID = %s, want 1234567890", resp.RequestID.String())
	}
}

func TestNas_UInfo_WithVipVersion(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("vip_version") != "v2" {
			t.Errorf("vip_version = %q, want v2", q.Get("vip_version"))
		}
		w.Write([]byte(`{"errno":0,"baidu_name":"user","netdisk_name":"user","avatar_url":"","vip_type":1,"uk":1,"request_id":1}`))
	})
	defer ts.Close()

	resp, err := c.Nas.UInfo(ctx(), &UInfoParams{
		VipVersion: Ptr("v2"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VipType != 1 {
		t.Errorf("VipType = %d, want 1", resp.VipType)
	}
}

func TestNas_UInfo_NilParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("vip_version") != "" {
			t.Errorf("vip_version should be empty, got %q", q.Get("vip_version"))
		}
		w.Write([]byte(`{"errno":0,"baidu_name":"user","netdisk_name":"user","avatar_url":"","vip_type":0,"uk":1,"request_id":1}`))
	})
	defer ts.Close()

	resp, err := c.Nas.UInfo(ctx(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.VipType != 0 {
		t.Errorf("VipType = %d, want 0", resp.VipType)
	}
}

func TestNas_UInfo_APIError(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"access denied"}`))
	})
	defer ts.Close()

	_, err := c.Nas.UInfo(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}
