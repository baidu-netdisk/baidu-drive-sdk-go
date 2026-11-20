package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkClientUInfoLoopback(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errno":0,"uk":12345,"baidu_name":"bench","vip_type":1}`))
	}))
	defer ts.Close()
	c := NewClient(WithBaseURL(ts.URL))
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Nas.UInfo(ctx, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClientListLoopback(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errno":0,"list":[{"fs_id":1,"path":"/bench/file.txt","server_filename":"file.txt","size":1024,"isdir":0}],"request_id":1}`))
	}))
	defer ts.Close()
	c := NewClient(WithBaseURL(ts.URL))
	ctx := context.Background()
	params := &ListParams{Dir: "/bench", Limit: Ptr(20)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.File.List(ctx, params); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkClientPrecreateLoopback(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errno":0,"uploadid":"bench-upload","block_list":[0],"return_type":1}`))
	}))
	defer ts.Close()
	c := NewClient(WithBaseURL(ts.URL))
	ctx := context.Background()
	params := &PrecreateParams{Path: "/bench/file.bin", Size: 1024, BlockList: []string{"d41d8cd98f00b204e9800998ecf8427e"}}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := c.Upload.Precreate(ctx, params); err != nil {
			b.Fatal(err)
		}
	}
}
