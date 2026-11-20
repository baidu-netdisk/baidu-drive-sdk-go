package scene

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baidu-netdisk/baidu-drive-sdk-go/baidudriver/api"
)

func BenchmarkSceneListDirLoopback(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errno":0,"list":[{"fs_id":1,"path":"/bench/file.txt","server_filename":"file.txt","size":1024,"isdir":0,"category":4}],"request_id":1}`))
	}))
	defer ts.Close()
	sc := New(api.NewClient(api.WithBaseURL(ts.URL)))
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sc.ListDir(ctx, "/bench", &ListDirOptions{Limit: 20}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSceneSearchLoopback(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/2.0/xpan/nas":
			_, _ = w.Write([]byte(`{"errno":0,"uk":12345}`))
		case "/xpan/unisearch":
			_, _ = w.Write([]byte(`{"data":[{"source":1,"list":[{"fsid":1,"filename":"file.txt","path":"/bench/file.txt","isdir":0,"category":4,"size":1024}]}],"error_no":0,"is_end":true,"request_id":1}`))
		default:
			b.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()
	sc := New(api.NewClient(api.WithBaseURL(ts.URL)))
	ctx := context.Background()
	params := &SearchParams{Query: "file", Dir: "/bench", Num: 20}
	if _, err := sc.Search(ctx, params); err != nil {
		b.Fatalf("warm-up search: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sc.Search(ctx, params); err != nil {
			b.Fatal(err)
		}
	}
}
