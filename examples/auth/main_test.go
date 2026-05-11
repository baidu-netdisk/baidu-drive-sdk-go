package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func TestGetDeviceCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("client_id") != "test_app_key" {
			t.Errorf("client_id = %q, want test_app_key", r.URL.Query().Get("client_id"))
		}
		w.Write([]byte(`{
			"device_code": "test_device_code",
			"user_code": "TEST1234",
			"verification_url": "https://example.com/verify",
			"qrcode_url": "https://example.com/qrcode",
			"expires_in": 300,
			"interval": 5
		}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	dc, err := c.Auth.DeviceCode(context.Background(), "test_app_key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dc.DeviceCode != "test_device_code" {
		t.Errorf("device_code = %q, want test_device_code", dc.DeviceCode)
	}
	if dc.UserCode != "TEST1234" {
		t.Errorf("user_code = %q, want TEST1234", dc.UserCode)
	}
}

func TestGetDeviceToken(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			// 模拟 authorization_pending 状态
			w.Write([]byte(`{"errno":-1,"errmsg":"authorization_pending"}`))
			return
		}
		w.Write([]byte(`{
			"access_token": "test_access_token",
			"refresh_token": "test_refresh_token",
			"expires_in": 3600
		}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	ctx := context.Background()

	// 测试轮询逻辑
	interval := 1 * time.Second
	deadline := time.Now().Add(5 * time.Second)

	var token *api.DeviceTokenResponse
	for time.Now().Before(deadline) {
		time.Sleep(interval)

		var err error
		token, err = c.Auth.DeviceToken(ctx, "test_app_key", "test_secret", "test_device_code")
		if err != nil {
			if api.IsErrno(err, -1) {
				continue
			}
			t.Fatalf("unexpected error: %v", err)
		}
		break
	}

	if token == nil {
		t.Fatal("expected token to be set")
	}
	if token.AccessToken != "test_access_token" {
		t.Errorf("access_token = %q, want test_access_token", token.AccessToken)
	}
}

func TestPollForToken_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"access_token": "test_token",
			"refresh_token": "test_refresh",
			"expires_in": 3600
		}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	ctx := context.Background()

	token, err := c.Auth.DeviceToken(ctx, "app", "secret", "device")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token.AccessToken != "test_token" {
		t.Errorf("access_token = %q, want test_token", token.AccessToken)
	}
}

func TestPollForToken_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":-6,"errmsg":"invalid device code"}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	ctx := context.Background()

	_, err := c.Auth.DeviceToken(ctx, "app", "secret", "device")
	if err == nil {
		t.Fatal("expected error for invalid device code")
	}
	if !api.IsErrno(err, -6) {
		t.Errorf("expected errno=-6, got: %v", err)
	}
}
