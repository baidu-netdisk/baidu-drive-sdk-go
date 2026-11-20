package api

import (
	"context"
	"net/http"
	"testing"
)

func TestFile_UniSearch_ValidatesDocumentedBounds(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("invalid parameters sent a request: %s", r.URL.String())
	})
	defer ts.Close()

	cases := []struct {
		name   string
		params *UniSearchParams
	}{
		{"scene", &UniSearchParams{Query: "x", Scene: "other"}},
		{"num too large", &UniSearchParams{Query: "x", Num: Ptr(501)}},
		{"num zero", &UniSearchParams{Query: "x", Num: Ptr(0)}},
		{"stream", &UniSearchParams{Query: "x", Stream: Ptr(2)}},
		{"search type", &UniSearchParams{Query: "x", SearchType: Ptr(3)}},
		{"category", &UniSearchParams{Query: "x", Category: []int{8}}},
		{"dir uk", &UniSearchParams{Query: "x", Dirs: []UniSearchDir{{Path: "/docs"}}}},
		{"dir path", &UniSearchParams{Query: "x", Dirs: []UniSearchDir{{UK: 1, Path: "docs"}}}},
		{"whitespace query", &UniSearchParams{Query: " \t"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.File.UniSearch(context.Background(), tc.params); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestFile_UniSearch_DecodesPIDWithoutMutatingScene(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("scene"); got != "mcpserver" {
			t.Errorf("scene = %q, want mcpserver", got)
		}
		_, _ = w.Write([]byte(`{"error_no":0,"data":[{"source":7,"list":[{"fsid":1,"filename":"doc.pdf","pid":99}]}]}`))
	})
	defer ts.Close()

	params := &UniSearchParams{Query: "doc"}
	resp, err := c.File.UniSearch(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if params.Scene != "" {
		t.Fatalf("input Scene was mutated to %q", params.Scene)
	}
	if got := resp.Files()[0].PID; got != 99 {
		t.Fatalf("PID = %d, want 99", got)
	}
}
