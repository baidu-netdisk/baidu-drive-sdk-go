package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func TestBuildDirs_NoDir(t *testing.T) {
	c := api.NewClient()
	dirs := buildDirs(context.Background(), c, []string{"cmd"})
	if dirs != nil {
		t.Errorf("expected nil, got %v", dirs)
	}
}

func TestBuildDirs_WithDir(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errno":0,"uk":2871298235,"baidu_name":"test","netdisk_name":"test"}`))
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	dirs := buildDirs(context.Background(), c, []string{"cmd", "query", "/docs"})
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d", len(dirs))
	}
	if dirs[0].UK != 2871298235 {
		t.Errorf("UK = %d, want 2871298235", dirs[0].UK)
	}
	if dirs[0].Path != "/docs" {
		t.Errorf("Path = %q, want /docs", dirs[0].Path)
	}
}

func TestBuildDirs_UInfoFails(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := api.NewClient(api.WithBaseURL(ts.URL))
	dirs := buildDirs(context.Background(), c, []string{"cmd", "query", "/docs"})
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %d", len(dirs))
	}
	if dirs[0].UK != 0 {
		t.Errorf("UK = %d, want 0 when UInfo fails", dirs[0].UK)
	}
	if dirs[0].Path != "/docs" {
		t.Errorf("Path = %q, want /docs", dirs[0].Path)
	}
}
