package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func TestRun_MissingToken(t *testing.T) {
	getenv := func(string) string { return "" }
	err := run(getenv, []string{"cmd"})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestRun_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"errno": 0,
			"total": 2199023255552,
			"used": 1099511627776,
			"expire": false,
			"request_id": "123"
		}`))
	}))
	defer ts.Close()

	// run() 内部用 WithAccessToken 创建客户端，无法指向 mock server，
	// 所以直接测试 runWithClient。
	client := api.NewClient(api.WithBaseURL(ts.URL))
	if err := runWithClient(client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithClient_Expire(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"errno": 0,
			"total": 2199023255552,
			"used": 2000000000000,
			"expire": true,
			"request_id": "456"
		}`))
	}))
	defer ts.Close()

	client := api.NewClient(api.WithBaseURL(ts.URL))
	if err := runWithClient(client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunWithClient_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := api.NewClient(api.WithBaseURL(ts.URL))
	if err := runWithClient(client); err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
		{2199023255552, "2.00 TB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
