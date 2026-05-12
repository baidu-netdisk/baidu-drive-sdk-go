package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/scene"
)

// newRouterServer 创建一个根据请求路径和 method 参数分发的测试服务器。
func newRouterServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Path + "?" + r.URL.Query().Get("method")
		h, ok := handlers[key]
		if !ok {
			t.Errorf("unexpected request: %s (key=%s)", r.URL.String(), key)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		h(w, r)
	}))
}

func TestRun_MissingToken(t *testing.T) {
	getenv := func(string) string { return "" }
	err := run(getenv, []string{"cmd"})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestRun_MissingDeviceID(t *testing.T) {
	getenv := func(string) string { return "fake_token" }
	err := run(getenv, []string{"cmd"})
	if err == nil {
		t.Fatal("expected error for missing device_id")
	}
}

func TestRunWithScene_Success(t *testing.T) {
	ts := newRouterServer(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{
				"errno": 0,
				"uk": 123456789,
				"baidu_name": "test_user",
				"netdisk_name": "test_netdisk",
				"avatar_url": "https://example.com/avatar.jpg",
				"vip_type": 2
			}`))
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("device_id") != "test_device" {
				t.Errorf("device_id = %q, want test_device", r.URL.Query().Get("device_id"))
			}
			w.Write([]byte(`{
				"error_code": 0,
				"data": {
					"has_privilege": 1,
					"is_svip": 1,
					"is_iot_svip": 1,
					"start_time": 1700000000,
					"end_time": 1730000000,
					"now": 1715000000
				}
			}`))
		},
	})
	defer ts.Close()

	client := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(client)

	if err := runWithScene(sc, "test_device"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithScene_IoTDataNil(t *testing.T) {
	ts := newRouterServer(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{
				"errno": 0,
				"uk": 987654321,
				"baidu_name": "user2",
				"netdisk_name": "netdisk2",
				"avatar_url": "https://example.com/avatar2.jpg"
			}`))
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"error_code": 0}`))
		},
	})
	defer ts.Close()

	client := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(client)

	if err := runWithScene(sc, "device2"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithScene_UInfoError(t *testing.T) {
	ts := newRouterServer(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"error_code": 0, "data": {}}`))
		},
	})
	defer ts.Close()

	client := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(client)

	if err := runWithScene(sc, "device"); err == nil {
		t.Fatal("expected error when UInfo fails")
	}
}

func TestRunWithScene_IoTError(t *testing.T) {
	ts := newRouterServer(t, map[string]http.HandlerFunc{
		"/rest/2.0/xpan/nas?uinfo": func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{
				"errno": 0,
				"uk": 111,
				"baidu_name": "u",
				"netdisk_name": "n",
				"avatar_url": "https://example.com/a.jpg"
			}`))
		},
		"/rest/2.0/xpan/device?iotqueryuinfo": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		},
	})
	defer ts.Close()

	client := api.NewClient(api.WithBaseURL(ts.URL))
	sc := scene.New(client)

	if err := runWithScene(sc, "device"); err == nil {
		t.Fatal("expected error when IoTQueryUInfo fails")
	}
}

func TestYesNo(t *testing.T) {
	if got := yesNo(1); got != "是" {
		t.Errorf("yesNo(1) = %q, want 是", got)
	}
	if got := yesNo(0); got != "否" {
		t.Errorf("yesNo(0) = %q, want 否", got)
	}
	if got := yesNo(2); got != "否" {
		t.Errorf("yesNo(2) = %q, want 否", got)
	}
}

func TestFormatTime(t *testing.T) {
	got := formatTime(1700000000)
	if got == "" {
		t.Error("formatTime returned empty string")
	}
}
