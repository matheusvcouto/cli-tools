package notionapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ne "github.com/matheusvcouto/cli-tools/internal/notionexport"
)

func id(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }

func envelope(block map[string]any) map[string]any {
	return map[string]any{"spaceId": "s", "value": map[string]any{"value": block, "role": "reader"}}
}

type fakeNotion struct {
	mu       sync.Mutex
	blocks   map[string]map[string]any // available via syncRecordValues
	chunk    map[string]map[string]any // returned by loadPageChunk
	calls    map[string]int
	bodies   map[string][]json.RawMessage
	headers  []http.Header
	status   map[string][]int // scripted statuses per endpoint
	signFail bool
}

func (f *fakeNotion) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	endpoint := strings.TrimPrefix(r.URL.Path, "/api/v3/")
	f.calls[endpoint]++
	f.headers = append(f.headers, r.Header.Clone())
	body, _ := io.ReadAll(r.Body)
	f.bodies[endpoint] = append(f.bodies[endpoint], body)
	if codes := f.status[endpoint]; len(codes) > 0 {
		f.status[endpoint] = codes[1:]
		w.WriteHeader(codes[0])
		return
	}
	switch endpoint {
	case "loadPageChunk":
		blocks := map[string]any{}
		for k, v := range f.chunk {
			blocks[k] = envelope(v)
		}
		json.NewEncoder(w).Encode(map[string]any{"recordMap": map[string]any{"block": blocks}, "cursor": map[string]any{"stack": []any{}}})
	case "syncRecordValuesMain":
		var req struct {
			Requests []struct{ ID string } `json:"requests"`
		}
		json.Unmarshal(body, &req)
		blocks := map[string]any{}
		for _, r := range req.Requests {
			if b, ok := f.blocks[r.ID]; ok {
				blocks[r.ID] = envelope(b)
			} else {
				blocks[r.ID] = map[string]any{"value": map[string]any{"role": "none"}}
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"recordMap": map[string]any{"block": blocks}})
	case "getSignedFileUrls":
		if f.signFail {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var req struct {
			URLs []struct {
				URL string `json:"url"`
			} `json:"urls"`
		}
		json.Unmarshal(body, &req)
		var urls []string
		for _, u := range req.URLs {
			urls = append(urls, "https://file.notion.so/signed/"+strings.ReplaceAll(u.URL, ":", "_")+"?signature=s")
		}
		json.NewEncoder(w).Encode(map[string]any{"signedUrls": urls})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func newFake() *fakeNotion {
	root := map[string]any{"id": id(1), "type": "page", "parent_table": "space", "properties": map[string]any{"title": [][]any{{"Raiz"}}}, "content": []string{id(2), id(3), id(4), id(5)}}
	f := &fakeNotion{calls: map[string]int{}, bodies: map[string][]json.RawMessage{}, status: map[string][]int{},
		chunk:  map[string]map[string]any{id(1): root, id(2): {"id": id(2), "type": "text", "content": []string{id(6)}}},
		blocks: map[string]map[string]any{}}
	f.blocks[id(3)] = map[string]any{"id": id(3), "type": "image", "properties": map[string]any{"source": [][]any{{"attachment:abc:foto.png"}}}}
	f.blocks[id(4)] = map[string]any{"id": id(4), "type": "transclusion_reference", "format": map[string]any{"transclusion_reference_pointer": map[string]any{"id": id(7)}}}
	f.blocks[id(7)] = map[string]any{"id": id(7), "type": "transclusion_container", "content": []string{id(8)}}
	f.blocks[id(8)] = map[string]any{"id": id(8), "type": "pdf", "properties": map[string]any{"source": [][]any{{"https://s3-us-west-2.amazonaws.com/secure.notion-static.com/x/doc.pdf"}}}}
	f.blocks[id(6)] = map[string]any{"id": id(6), "type": "text", "properties": map[string]any{"title": [][]any{{"‣", [][]any{{"p", id(9)}}}}}}
	f.blocks[id(9)] = map[string]any{"id": id(9), "type": "page", "properties": map[string]any{"title": [][]any{{"Mencionada"}}}, "content": []string{id(10)}}
	// id(5) is never returned (no permission) and id(10) is child-page content.
	f.blocks[id(10)] = map[string]any{"id": id(10), "type": "image"}
	return f
}

func testClient(srv *httptest.Server) (Client, *[]string) {
	var warnings []string
	return Client{HTTP: srv.Client(), BaseURL: srv.URL + "/api/v3", Retry: ne.RetryPolicy{MaxRetries: 2, Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }},
		Warn: func(s string) { warnings = append(warnings, s) }}, &warnings
}

func TestFetchPageLoadsMissingBlocksAndSignsFiles(t *testing.T) {
	fake := newFake()
	srv := httptest.NewServer(fake)
	defer srv.Close()
	client, warnings := testClient(srv)
	rm, err := client.FetchPage(context.Background(), ne.CompactID(id(1)))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{3, 4, 6, 7, 8, 9} {
		if !rm.HasBlock(id(n)) {
			t.Errorf("block %d not fetched", n)
		}
	}
	if rm.HasBlock(id(10)) {
		t.Fatal("child page content must not be fetched")
	}
	if fake.calls["syncRecordValuesMain"] > 4 {
		t.Fatalf("missing-block loop not bounded: %d calls", fake.calls["syncRecordValuesMain"])
	}
	if got := rm.SignedURL(id(3)); !strings.HasPrefix(got, "https://file.notion.so/signed/attachment_abc_foto.png") {
		t.Fatalf("signed image = %q", got)
	}
	if rm.SignedURL(id(8)) == "" {
		t.Fatal("legacy S3 source not signed")
	}
	if len(*warnings) != 0 {
		t.Fatalf("warnings: %v", *warnings)
	}
	var chunk map[string]any
	json.Unmarshal(fake.bodies["loadPageChunk"][0], &chunk)
	if chunk["pageId"] != id(1) || chunk["limit"].(float64) != 100 {
		t.Fatalf("loadPageChunk body %v", chunk)
	}
	for _, h := range fake.headers {
		if h.Get("Cookie") != "" || h.Get("Authorization") != "" || h.Get("Content-Type") != "application/json" || !strings.Contains(h.Get("User-Agent"), "notion-export") {
			t.Fatalf("unexpected request headers %v", h)
		}
	}
	page := rm
	root, _ := page.RootPage(id(1))
	assets := ne.CollectAssets(page, root)
	if len(assets) != 2 || assets[0].Type != ne.AssetImage || assets[1].Type != ne.AssetPDF {
		t.Fatalf("assets %+v", assets)
	}
}

func TestFetchPageErrorsAndRetries(t *testing.T) {
	fake := newFake()
	fake.status["loadPageChunk"] = []int{http.StatusServiceUnavailable, http.StatusTooManyRequests}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	client, _ := testClient(srv)
	if _, err := client.FetchPage(context.Background(), ne.CompactID(id(1))); err != nil {
		t.Fatalf("transient errors not retried: %v", err)
	}
	if fake.calls["loadPageChunk"] != 3 {
		t.Fatalf("calls = %d", fake.calls["loadPageChunk"])
	}
	fake.status["loadPageChunk"] = []int{http.StatusBadRequest}
	if _, err := client.FetchPage(context.Background(), ne.CompactID(id(1))); !errors.Is(err, ErrNotPublic) {
		t.Fatalf("400 => %v", err)
	}
	if _, err := client.FetchPage(context.Background(), ne.CompactID(id(99))); !errors.Is(err, ErrNotPublic) {
		t.Fatalf("missing page => %v", err)
	}
	fake.status["loadPageChunk"] = []int{500, 500, 500}
	var status *ne.StatusError
	if _, err := client.FetchPage(context.Background(), ne.CompactID(id(1))); !errors.As(err, &status) || status.StatusCode != 500 {
		t.Fatalf("exhausted retries => %v", err)
	}
}

func TestSigningFailureIsAWarning(t *testing.T) {
	fake := newFake()
	fake.signFail = true
	srv := httptest.NewServer(fake)
	defer srv.Close()
	client, warnings := testClient(srv)
	rm, err := client.FetchPage(context.Background(), ne.CompactID(id(1)))
	if err != nil {
		t.Fatal(err)
	}
	if len(*warnings) == 0 || rm.SignedURL(id(3)) != "" {
		t.Fatalf("warnings=%v", *warnings)
	}
	root, _ := rm.RootPage(id(1))
	// The unsigned attachment: source is not downloadable; the legacy S3 URL is.
	if assets := ne.CollectAssets(rm, root); len(assets) != 1 || assets[0].Type != ne.AssetPDF {
		t.Fatalf("assets %+v", assets)
	}
}

func TestOversizedAndInvalidResponsesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("not json")) }))
	defer srv.Close()
	client, _ := testClient(srv)
	if _, err := client.FetchPage(context.Background(), ne.CompactID(id(1))); err == nil || !strings.Contains(err.Error(), "JSON") {
		t.Fatalf("invalid JSON => %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.FetchPage(ctx, ne.CompactID(id(1))); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled => %v", err)
	}
}

func TestIsNotionFile(t *testing.T) {
	for _, s := range []string{"attachment:a:b.png", "https://file.notion.so/f/x", "https://file.notion.com/f/x", "https://prod-files-secure.s3.us-west-2.amazonaws.com/x", "https://s3-us-west-2.amazonaws.com/secure.notion-static.com/x/y.png", "https://img.notionusercontent.com/s3/x"} {
		if !IsNotionFile(s) {
			t.Error(s)
		}
	}
	for _, s := range []string{"https://images.unsplash.com/x", "https://s3.amazonaws.com/other/x", "https://notion.so.evil.example/x", ""} {
		if IsNotionFile(s) {
			t.Error(s)
		}
	}
}
