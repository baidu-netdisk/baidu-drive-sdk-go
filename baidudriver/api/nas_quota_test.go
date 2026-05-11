package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// Quota 接口测试
// =============================================================================

func TestNasService_Quota(t *testing.T) {
	// 模拟成功响应
	mockResponse := map[string]interface{}{
		"errno":      0,
		"errmsg":     "succ",
		"request_id": "4890482559098510375",
		"total":      2205465706496,
		"used":       686653888910,
		"free":       2205465706496,
		"expire":     false,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		if r.URL.Path != "/api/quota" {
			t.Errorf("path = %q, want /api/quota", r.URL.Path)
		}
		// 验证请求方法
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	resp, err := client.Nas.Quota(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Errno != 0 {
		t.Errorf("errno = %d, want 0", resp.Errno)
	}
	if resp.Total != 2205465706496 {
		t.Errorf("total = %d, want 2205465706496", resp.Total)
	}
	if resp.Used != 686653888910 {
		t.Errorf("used = %d, want 686653888910", resp.Used)
	}
	if resp.Expire != false {
		t.Errorf("expire = %v, want false", resp.Expire)
	}
}

func TestNasService_Quota_WithParams(t *testing.T) {
	mockResponse := map[string]interface{}{
		"errno":      0,
		"errmsg":     "succ",
		"request_id": "4890482559098510376",
		"total":      2205465706496,
		"used":       686653888910,
		"free":       1078821817086,
		"expire":     true,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证查询参数
		q := r.URL.Query()
		if q.Get("checkfree") != "1" {
			t.Errorf("checkfree = %q, want 1", q.Get("checkfree"))
		}
		if q.Get("checkexpire") != "1" {
			t.Errorf("checkexpire = %q, want 1", q.Get("checkexpire"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	checkfree := 1
	checkexpire := 1
	resp, err := client.Nas.Quota(context.Background(), &QuotaParams{
		CheckFree:   &checkfree,
		CheckExpire: &checkexpire,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Free != 1078821817086 {
		t.Errorf("free = %d, want 1078821817086", resp.Free)
	}
	if resp.Expire != true {
		t.Errorf("expire = %v, want true", resp.Expire)
	}
}

func TestNasService_Quota_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errno":  -6,
			"errmsg": "身份验证失败",
		})
	}))
	defer server.Close()

	client := NewClient(WithBaseURL(server.URL))
	_, err := client.Nas.Quota(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Errno != -6 {
		t.Errorf("errno = %d, want -6", apiErr.Errno)
	}
}

// ExampleNasService_Quota 演示如何使用 Quota 接口。
func ExampleNasService_Quota() {
	// 实际使用时，请替换为真实的 access_token
	client := NewClient(WithAccessToken("your_access_token"))

	resp, err := client.Nas.Quota(context.Background(), nil)
	if err != nil {
		fmt.Println("获取容量信息失败:", err)
		return
	}

	fmt.Printf("总容量: %d bytes\n", resp.Total)
	fmt.Printf("已使用: %d bytes\n", resp.Used)
	fmt.Printf("剩余可用: %d bytes\n", resp.Total-resp.Used)
	fmt.Printf("是否有容量即将过期: %v\n", resp.Expire)
}
