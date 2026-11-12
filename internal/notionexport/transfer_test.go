package notionexport

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

func noSleep(ctx context.Context, _ time.Duration) error { return ctx.Err() }

func testRetry(n int) RetryPolicy {
	return RetryPolicy{MaxRetries: n, BaseDelay: time.Millisecond, MaxDelay: time.Millisecond, RateLimitDelay: time.Millisecond, Sleep: noSleep}
}

func openTestRoot(t *testing.T) (*safefs.Root, string) {
	t.Helper()
	dir := t.TempDir()
	root, err := safefs.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root, dir
}

func TestProbeSizesUsesRangeAndFallsBack(t *testing.T) {
	var sawRange atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusForbidden) // like file.notion.so for HEAD
			return
		}
		switch r.URL.Path {
		case "/range":
			if r.Header.Get("Range") == "bytes=0-0" {
				sawRange.Add(1)
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Range", "bytes 0-0/1749732")
			w.WriteHeader(http.StatusPartialContent)
			w.Write([]byte{0})
		case "/full":
			w.Header().Set("Content-Length", "42")
			w.Write(bytes.Repeat([]byte{1}, 42))
		case "/star":
			w.Header().Set("Content-Range", "bytes 0-0/*")
			w.WriteHeader(http.StatusPartialContent)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()
	assets := []Asset{{URL: srv.URL + "/range"}, {URL: srv.URL + "/full"}, {URL: srv.URL + "/star"}, {URL: srv.URL + "/denied"}, {URL: "http://127.0.0.1:1/unreachable"}}
	var calls atomic.Int32
	got := ProbeSizes(context.Background(), srv.Client(), assets, 3, func(done, total int) { calls.Add(1) })
	if got[0].EstimatedSize == nil || *got[0].EstimatedSize != 1749732 || got[0].ContentType != "image/png" || sawRange.Load() != 1 {
		t.Fatalf("range probe: %+v", got[0])
	}
	if got[1].EstimatedSize == nil || *got[1].EstimatedSize != 42 {
		t.Fatalf("content-length fallback: %+v", got[1])
	}
	for _, i := range []int{2, 3, 4} {
		if got[i].EstimatedSize != nil {
			t.Fatalf("asset %d should be unknown: %v", i, *got[i].EstimatedSize)
		}
	}
	if assets[0].EstimatedSize != nil || calls.Load() != 5 {
		t.Fatal("input mutated or progress missing")
	}
	s := Summarize(got)
	if s[""].Count != 5 || s[""].UnknownCount != 3 || s[""].KnownBytes != 1749774 {
		t.Fatalf("summary %+v", s[""])
	}
}

func assetServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	var flaky atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/img.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write([]byte("\x89PNG-binary\x00\xff"))
		case "/noext":
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte("jpeg"))
		case "/login.mp4":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>login</html>"))
		case "/truncated.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Header().Set("Content-Length", "1000")
			w.Write([]byte("%PDF-short"))
		case "/flaky.mp3":
			if flaky.Add(1) < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write([]byte("ID3"))
		case "/missing.png":
			w.WriteHeader(http.StatusNotFound)
		case "/stall.png":
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Content-Length", "10")
			w.Write([]byte("12"))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &flaky
}

func TestDownloaderOutcomesAreIsolatedAndClean(t *testing.T) {
	srv, _ := assetServer(t)
	root, dir := openTestRoot(t)
	assets := []Asset{
		{Index: 1, BlockID: "a", Type: AssetImage, Source: "https://notion.example/Foto Ótima.png", URL: srv.URL + "/img.png"},
		{Index: 2, BlockID: "b", Type: AssetImage, URL: srv.URL + "/noext"},
		{Index: 3, BlockID: "c", Type: AssetVideo, URL: srv.URL + "/login.mp4"},
		{Index: 4, BlockID: "d", Type: AssetPDF, URL: srv.URL + "/truncated.pdf"},
		{Index: 5, BlockID: "e", Type: AssetAudio, URL: srv.URL + "/flaky.mp3"},
		{Index: 6, BlockID: "f", Type: AssetImage, URL: srv.URL + "/missing.png"},
		{Index: 7, BlockID: "g", Type: AssetFile, Source: "attachment:x:a.zip", URL: "attachment:x:a.zip"},
	}
	var events atomic.Int32
	d := Downloader{Client: srv.Client(), Retry: testRetry(2), Concurrency: 4, Progress: func(DownloadEvent) { events.Add(1) }}
	res := d.Download(context.Background(), root, assets)
	want := map[string]DownloadStatus{"a": StatusDownloaded, "b": StatusDownloaded, "c": StatusFailed, "d": StatusFailed, "e": StatusDownloaded, "f": StatusFailed, "g": StatusFailed}
	for id, status := range want {
		if res[id].Status != status {
			t.Errorf("%s: %+v", id, res[id])
		}
	}
	if res["a"].Path != "assets/images/001-Foto-Otima.png" || res["b"].Path != "assets/images/002-noext.jpg" || res["e"].Path != "assets/audio/005-flaky.mp3" {
		t.Fatalf("paths: %q %q %q", res["a"].Path, res["b"].Path, res["e"].Path)
	}
	data, err := os.ReadFile(filepath.Join(dir, "assets", "images", "001-Foto-Otima.png"))
	if err != nil || string(data) != "\x89PNG-binary\x00\xff" || res["a"].Bytes != int64(len(data)) {
		t.Fatalf("binary content: %q %v", data, err)
	}
	if !strings.Contains(res["c"].Err.Error(), "Content-Type incompatível") || !strings.Contains(res["d"].Err.Error(), "incompleto") {
		t.Fatalf("errors: %v / %v", res["c"].Err, res["d"].Err)
	}
	var status *StatusError
	if !errors.As(res["f"].Err, &status) || status.StatusCode != 404 {
		t.Fatalf("404 error: %v", res["f"].Err)
	}
	// Failed transfers leave no partial files behind.
	for _, name := range []string{"videos/003-login.mp4", "pdfs/004-truncated.pdf"} {
		if _, err := os.Lstat(filepath.Join(dir, "assets", filepath.FromSlash(name))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("partial file %s remains: %v", name, err)
		}
	}
	if events.Load() != int32(2*len(assets)) {
		t.Fatalf("events=%d, want start+finish per asset", events.Load())
	}
}

func TestDownloaderNeverReplacesExistingFile(t *testing.T) {
	srv, _ := assetServer(t)
	root, dir := openTestRoot(t)
	if err := os.MkdirAll(filepath.Join(dir, "assets", "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "assets", "images", "001-img.png")
	if err := os.WriteFile(target, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := Downloader{Client: srv.Client(), Concurrency: 1}.Download(context.Background(), root, []Asset{{Index: 1, BlockID: "a", Type: AssetImage, URL: srv.URL + "/img.png"}})
	if res["a"].Status != StatusFailed {
		t.Fatal("existing file replaced")
	}
	if data, _ := os.ReadFile(target); string(data) != "keep" {
		t.Fatal("existing file modified or removed")
	}
}

func TestDownloaderStallTimeoutAndCancellation(t *testing.T) {
	srv, _ := assetServer(t)
	root, dir := openTestRoot(t)
	d := Downloader{Client: srv.Client(), Concurrency: 1, StallTimeout: 50 * time.Millisecond}
	res := d.Download(context.Background(), root, []Asset{{Index: 1, BlockID: "s", Type: AssetImage, URL: srv.URL + "/stall.png"}})
	if res["s"].Status != StatusFailed || !errors.Is(res["s"].Err, errStalled) {
		t.Fatalf("stall: %+v", res["s"])
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	res = d.Download(ctx, root, []Asset{{Index: 2, BlockID: "x", Type: AssetImage, URL: srv.URL + "/img.png"}})
	if res["x"].Status != StatusFailed || !errors.Is(res["x"].Err, context.Canceled) {
		t.Fatalf("cancel: %+v", res["x"])
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "assets", "images"))
	if len(entries) != 0 {
		t.Fatalf("leftover files: %v", entries)
	}
}

func TestRetryPolicy(t *testing.T) {
	var attempts int
	err := testRetry(3).Do(context.Background(), func(int) (bool, error) {
		attempts++
		return true, &StatusError{StatusCode: 503}
	})
	if attempts != 4 || err == nil {
		t.Fatalf("attempts=%d err=%v", attempts, err)
	}
	attempts = 0
	_ = testRetry(3).Do(context.Background(), func(int) (bool, error) { attempts++; return false, errors.New("permanent") })
	if attempts != 1 {
		t.Fatal("permanent error retried")
	}
	p := RetryPolicy{MaxRetries: 1, BaseDelay: time.Second, MaxDelay: time.Second, RateLimitDelay: time.Minute}
	if d := p.delay(0, &StatusError{StatusCode: 429}); d < time.Minute || d > time.Minute+15*time.Second {
		t.Fatalf("429 delay %v", d)
	}
	if d := p.delay(0, &StatusError{StatusCode: 503, RetryAfter: 7 * time.Second}); d != 7*time.Second {
		t.Fatalf("Retry-After delay %v", d)
	}
	if parseRetryAfter("9999") != maxRetryAfter || parseRetryAfter("2") != 2*time.Second || parseRetryAfter("x") != 0 {
		t.Fatal("parseRetryAfter")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	attempts = 0
	err = DefaultRetryPolicy().Do(ctx, func(int) (bool, error) { attempts++; return true, errors.New("x") })
	if attempts != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled retry: %d %v", attempts, err)
	}
	for _, code := range []int{408, 429, 500, 502, 503, 504} {
		if !RetryableStatus(code) {
			t.Error(code)
		}
	}
	for _, code := range []int{400, 401, 403, 404, 409} {
		if RetryableStatus(code) {
			t.Error(code)
		}
	}
}

func TestRedirectPolicy(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "file:///etc/passwd", nil)
	if checkRedirect(req, nil) == nil {
		t.Fatal("file redirect allowed")
	}
	ok, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	if checkRedirect(ok, make([]*http.Request, maxRedirects)) == nil || checkRedirect(ok, nil) != nil {
		t.Fatal("redirect limit")
	}
}

func TestPermanentTransportErrorsAreNotRetried(t *testing.T) {
	ctx := context.Background()
	for _, err := range []error{
		&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}},
		fmt.Errorf("get: %w", x509.HostnameError{Host: "x"}),
		fmt.Errorf("get: %w", errRedirectPolicy),
		context.Canceled,
	} {
		if TransientNetworkError(ctx, err) {
			t.Errorf("%v retried", err)
		}
	}
	if !TransientNetworkError(ctx, io.ErrUnexpectedEOF) {
		t.Fatal("connection reset must be retried")
	}
	var waits []time.Duration
	p := testRetry(2)
	p.Notify = func(wait time.Duration, _ error) { waits = append(waits, wait) }
	_ = p.Do(ctx, func(int) (bool, error) { return true, errors.New("x") })
	if len(waits) != 2 {
		t.Fatalf("Notify calls = %d", len(waits))
	}
}

func TestDownloaderReportsBytes(t *testing.T) {
	srv, _ := assetServer(t)
	root, _ := openTestRoot(t)
	var total atomic.Int64
	d := Downloader{Client: srv.Client(), Concurrency: 2, OnBytes: func(n int64) { total.Add(n) }}
	d.Download(context.Background(), root, []Asset{{Index: 1, BlockID: "a", Type: AssetImage, URL: srv.URL + "/img.png"}, {Index: 2, BlockID: "d", Type: AssetPDF, URL: srv.URL + "/truncated.pdf"}})
	if total.Load() != int64(len("\x89PNG-binary\x00\xff")) {
		t.Fatalf("bytes = %d; discarded transfers must be subtracted", total.Load())
	}
}
