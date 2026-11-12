package notioncli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	ne "github.com/matheusvcouto/cli-tools/internal/notionexport"
)

const pageID = "0123456789abcdef0123456789abcdef"

func uuid(n int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", n, n) }

// syntheticNotion serves a minimal public page plus its assets.
func syntheticNotion(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		env := func(b map[string]any) map[string]any { return map[string]any{"value": map[string]any{"value": b}} }
		switch r.URL.Path {
		case "/api/v3/loadPageChunk":
			json.NewEncoder(w).Encode(map[string]any{"recordMap": map[string]any{"block": map[string]any{
				ne.UUID(pageID): env(map[string]any{"id": ne.UUID(pageID), "type": "page", "parent_table": "space", "properties": map[string]any{"title": [][]any{{"Página \x1b[31mSintética"}}}, "content": []string{uuid(2), uuid(3), uuid(4)}}),
				uuid(2):         env(map[string]any{"id": uuid(2), "type": "text", "properties": map[string]any{"title": [][]any{{"Olá"}}}}),
				uuid(3):         env(map[string]any{"id": uuid(3), "type": "image", "properties": map[string]any{"source": [][]any{{srv.URL + "/files/foto.png"}}}}),
				uuid(4):         env(map[string]any{"id": uuid(4), "type": "pdf", "properties": map[string]any{"source": [][]any{{srv.URL + "/files/doc.pdf"}}}}),
			}}})
		case "/files/foto.png":
			w.Header().Set("Content-Type", "image/png")
			if r.Header.Get("Range") != "" {
				w.Header().Set("Content-Range", "bytes 0-0/4")
				w.WriteHeader(http.StatusPartialContent)
				w.Write([]byte("P"))
				return
			}
			w.Write([]byte("PNG!"))
		case "/files/doc.pdf":
			w.Header().Set("Content-Type", "application/pdf")
			w.Write([]byte("%PDF"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func testDeps(srv *httptest.Server) Dependencies {
	return Dependencies{HTTP: srv.Client(), BaseURL: srv.URL + "/api/v3",
		Retry: ne.RetryPolicy{Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }},
		Now:   func() time.Time { return time.Unix(0, 0) }}
}

type runResult struct {
	out, err string
	code     int
}

func runCLI(t *testing.T, deps Dependencies, stdin string, tty bool, args ...string) runResult {
	t.Helper()
	var out, errOut bytes.Buffer
	in := strings.NewReader(stdin)
	app, err := New(deps, core.ProductMetadata{Version: "0.0.0-test"}, core.TextInteraction{In: in, Out: &errOut, Interactive: tty})
	if err != nil {
		t.Fatal(err)
	}
	runErr := app.Run(context.Background(), args, core.IO{In: in, Out: &out, Err: &errOut, Terminal: core.Terminal{StdinTTY: tty}})
	if runErr != nil {
		core.RenderDiagnostic(&errOut, runErr)
	}
	return runResult{out.String(), errOut.String(), core.ExitCode(runErr)}
}

func TestStaticSurfaceNeedsNoNetwork(t *testing.T) {
	deps := Dependencies{BaseURL: "http://127.0.0.1:1/never"}
	r := runCLI(t, deps, "", false, "--help")
	if r.code != 0 || !strings.Contains(r.out, "--from-json") || !strings.Contains(r.out, "--force") {
		t.Fatalf("help: %+v", r)
	}
	if r := runCLI(t, deps, "", false, "--version"); r.out != "notion-export 0.0.0-test\n" {
		t.Fatalf("version: %q", r.out)
	}
}

func TestUsageErrorsHappenBeforeNetwork(t *testing.T) {
	srv, hits := syntheticNotion(t)
	deps := testDeps(srv)
	for _, args := range [][]string{
		{},
		{pageID, "--from-json", "page.json"},
		{"not-a-page-id"},
		{pageID, "--format", "pdf"},
		{pageID, "--images", "maybe"},
		{pageID, "--concurrency", "0"},
		{pageID, "--concurrency", "65"},
		{pageID, "--out", ""},
	} {
		if r := runCLI(t, deps, "", false, args...); r.code != 2 {
			t.Errorf("args %v: code=%d err=%s", args, r.code, r.err)
		}
	}
	if hits.Load() != 0 {
		t.Fatalf("usage errors reached the network %d times", hits.Load())
	}
}

func TestNonInteractiveExportTreatsAskAsNo(t *testing.T) {
	srv, _ := syntheticNotion(t)
	out := t.TempDir()
	r := runCLI(t, testDeps(srv), "", false, "https://example.notion.site/P-"+pageID, "--out", out)
	if r.code != 0 {
		t.Fatalf("code=%d err=%s", r.code, r.err)
	}
	dir := filepath.Join(out, "pagina-31msintetica")
	if r.out != dir+"\n" {
		t.Fatalf("stdout must only carry the export path: %q", r.out)
	}
	if strings.Contains(r.err, "\x1b") {
		t.Fatal("terminal escape from page title reached stderr")
	}
	if !strings.Contains(r.err, "Entrada não interativa") || !strings.Contains(r.err, "Imagens: 1 (4 B)") || !strings.Contains(r.err, "Assets ignorados: 2") {
		t.Fatalf("stderr: %s", r.err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets")); !os.IsNotExist(err) {
		t.Fatal("assets downloaded without consent")
	}
	// Second run: destination exists.
	if r := runCLI(t, testDeps(srv), "", false, pageID, "--out", out); r.code != 1 || !strings.Contains(r.err, "--force") {
		t.Fatalf("existing destination: %+v", r)
	}
	// --yes with a per-type override, replacing the previous export.
	r = runCLI(t, testDeps(srv), "", false, pageID, "--out", out, "--yes", "--pdfs", "no", "--force", "--format", "html")
	if r.code != 0 {
		t.Fatalf("force: %s", r.err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "images", "001-foto.png")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "pdfs")); !os.IsNotExist(err) {
		t.Fatal("--pdfs no must override --yes")
	}
	if _, err := os.Stat(filepath.Join(dir, "page.md")); !os.IsNotExist(err) {
		t.Fatal("--format html must not write Markdown")
	}
}

func TestInteractivePromptsPerAssetType(t *testing.T) {
	srv, _ := syntheticNotion(t)
	out := t.TempDir()
	r := runCLI(t, testDeps(srv), "sim\nn\n", true, pageID, "--out", out, "--no-size-probe")
	if r.code != 0 {
		t.Fatalf("code=%d err=%s", r.code, r.err)
	}
	if !strings.Contains(r.err, "Encontradas 1 imagem (tamanho total desconhecido). Baixar imagens? [s/N]") || !strings.Contains(r.err, "Encontrados 1 PDF") {
		t.Fatalf("prompts: %s", r.err)
	}
	dir := strings.TrimSpace(r.out)
	if _, err := os.Stat(filepath.Join(dir, "assets", "images", "001-foto.png")); err != nil {
		t.Fatal("image answer ignored")
	}
	if _, err := os.Stat(filepath.Join(dir, "assets", "pdfs")); !os.IsNotExist(err) {
		t.Fatal("PDF downloaded after a no")
	}
}

func TestFromJSONReprocessesSavedPage(t *testing.T) {
	srv, _ := syntheticNotion(t)
	first := t.TempDir()
	r := runCLI(t, testDeps(srv), "", false, pageID, "--out", first, "--yes")
	if r.code != 0 {
		t.Fatal(r.err)
	}
	second := t.TempDir()
	r = runCLI(t, testDeps(srv), "", false, "--from-json", filepath.Join(strings.TrimSpace(r.out), "page.json"), "--out", second, "--images", "yes", "--pdfs", "no")
	if r.code != 0 || !strings.HasPrefix(r.out, second) {
		t.Fatalf("from-json: %+v", r)
	}
	data, _ := os.ReadFile(filepath.Join(strings.TrimSpace(r.out), "manifest.json"))
	var m ne.Manifest
	if err := json.Unmarshal(data, &m); err != nil || m.Source != nil || m.PageID != pageID || m.Assets.Downloaded != 1 {
		t.Fatalf("manifest: %+v %v", m, err)
	}
	if r := runCLI(t, testDeps(srv), "", false, "--from-json", filepath.Join(second, "missing.json"), "--out", second); r.code != 1 {
		t.Fatalf("missing file: %+v", r)
	}
}

func TestUnavailablePageFailsClearly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) }))
	defer srv.Close()
	out := t.TempDir()
	r := runCLI(t, testDeps(srv), "", false, pageID, "--out", out)
	if r.code != 1 || !strings.Contains(r.err, "não pública") {
		t.Fatalf("%+v", r)
	}
	if entries, _ := os.ReadDir(out); len(entries) != 0 {
		t.Fatal("failed fetch wrote files")
	}
}

func TestHelpers(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 1023: "1023 B", 1536: "1.5 KiB", 15518924: "14.8 MiB", -1: "desconhecido"} {
		if got := FormatBytes(n); got != want {
			t.Errorf("FormatBytes(%d) = %q", n, got)
		}
	}
	if DescribeGroupSize(ne.GroupSummary{Count: 2, KnownBytes: 2048, UnknownCount: 1}) != "pelo menos 2.0 KiB; 1 com tamanho desconhecido" {
		t.Fatal("DescribeGroupSize")
	}
	for _, yes := range []string{"s", "SIM", " y ", "yes"} {
		if !isYes(yes) {
			t.Error(yes)
		}
	}
	for _, no := range []string{"", "n", "não", "nope"} {
		if isYes(no) {
			t.Error(no)
		}
	}
	if got := sanitize("a\x1b[2Jb\u009bc\n\u202egpj.exe\u200b"); got != "a[2Jbcgpj.exe" {
		t.Fatal(got)
	}
}

func TestContractLockIsCurrent(t *testing.T) {
	app, err := New(Dependencies{}, core.ProductMetadata{Version: "0.0.0"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.ContractJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../cmd/notion-export/cli.contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("notion-export cli.contract.json is stale; regenerate it from __cli contract")
	}
}

func TestNameFlagSeparatesPagesWithTheSameTitle(t *testing.T) {
	srv, hits := syntheticNotion(t)
	out := t.TempDir()
	if r := runCLI(t, testDeps(srv), "", false, pageID, "--out", out); r.code != 0 {
		t.Fatal(r.err)
	}
	r := runCLI(t, testDeps(srv), "", false, pageID, "--out", out, "--name", "copia_2")
	if r.code != 0 || r.out != filepath.Join(out, "copia_2")+"\n" {
		t.Fatalf("--name: %+v", r)
	}
	before := hits.Load()
	for _, bad := range []string{".hidden", "a/b", `a\\b`, "..", "nome com espaço", strings.Repeat("a", 81), ""} {
		if r := runCLI(t, testDeps(srv), "", false, pageID, "--out", out, "--name", bad); r.code != 2 {
			t.Errorf("--name %q: code=%d", bad, r.code)
		}
	}
	if hits.Load() != before {
		t.Fatal("invalid --name reached the network")
	}
}

func TestRateLimitWaitIsAnnounced(t *testing.T) {
	srv, _ := syntheticNotion(t)
	var limited atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/loadPageChunk" && limited.CompareAndSwap(false, true) {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		http.Redirect(w, r, srv.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer proxy.Close()
	deps := testDeps(srv)
	deps.BaseURL = proxy.URL + "/api/v3"
	deps.Retry = ne.RetryPolicy{MaxRetries: 1, RateLimitDelay: time.Second, Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}
	r := runCLI(t, deps, "", false, pageID, "--out", t.TempDir(), "--no-size-probe")
	if r.code != 0 || !strings.Contains(r.err, "HTTP 429") || !strings.Contains(r.err, "nova tentativa em") {
		t.Fatalf("%+v", r)
	}
}

func TestLiveProgressOnTerminal(t *testing.T) {
	srv, _ := syntheticNotion(t)
	var out, errOut bytes.Buffer
	app, err := New(testDeps(srv), core.ProductMetadata{Version: "0"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{pageID, "--out", t.TempDir(), "--yes"}, core.IO{Out: &out, Err: &errOut, Terminal: core.Terminal{StderrTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	s := errOut.String()
	if !strings.Contains(s, "\r\x1b[2KBaixando assets: 2/2 · 8 B de 8 B") || strings.Contains(s, "Downloads: ") {
		t.Fatalf("live progress: %q", s)
	}
}
