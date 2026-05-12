package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenTransport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("access_token")
		if token != "test_token" {
			t.Errorf("access_token = %q, want %q", token, "test_token")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	transport := &tokenTransport{
		token: "test_token",
		base:  http.DefaultTransport,
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestTokenTransport_PreservesExistingParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("access_token")
		method := r.URL.Query().Get("method")
		if token != "tk" {
			t.Errorf("access_token = %q, want %q", token, "tk")
		}
		if method != "list" {
			t.Errorf("method = %q, want %q", method, "list")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	transport := &tokenTransport{token: "tk", base: http.DefaultTransport}
	client := &http.Client{Transport: transport}
	resp, err := client.Get(ts.URL + "/test?method=list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}

func TestApiKeyTransport(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("api_key")
		if key != "test_api_key" {
			t.Errorf("api_key = %q, want %q", key, "test_api_key")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	transport := &apiKeyTransport{
		apiKey: "test_api_key",
		base:   http.DefaultTransport,
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(ts.URL + "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestApiKeyTransport_PreservesExistingParams(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("api_key")
		method := r.URL.Query().Get("method")
		if key != "mykey" {
			t.Errorf("api_key = %q, want %q", key, "mykey")
		}
		if method != "list" {
			t.Errorf("method = %q, want %q", method, "list")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	transport := &apiKeyTransport{apiKey: "mykey", base: http.DefaultTransport}
	client := &http.Client{Transport: transport}
	resp, err := client.Get(ts.URL + "/test?method=list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
}
