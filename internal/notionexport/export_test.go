package notionexport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

type fakeSource struct {
	rm  *RecordMap
	err error
}

func (f fakeSource) FetchPage(context.Context, string) (*RecordMap, error) { return f.rm, f.err }

func exportFixture(t *testing.T, assetURL string) (*fixture, Service) {
	f := newFixture(t, "Minha Página", testID(2), testID(3), testID(4))
	f.block(testID(2), "text", map[string]any{"title": text("Olá")})
	f.block(testID(3), "image", map[string]any{"source": text("attachment:x:foto.png"), "caption": text("Foto")})
	f.signed[testID(3)] = assetURL + "/img.png"
	f.block(testID(4), "video", map[string]any{"source": text(assetURL + "/login.mp4")})
	svc := Service{Source: fakeSource{rm: f.recordMap()}, Retry: testRetry(0),
		Now:       func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 6e6, time.UTC) },
		Generator: Generator{Name: GeneratorName, Version: "9.9.9"}}
	return f, svc
}

func fetchFixturePage(t *testing.T, svc Service) *Page {
	t.Helper()
	page, err := svc.Fetch(context.Background(), "https://example.notion.site/Minha-Pagina-"+CompactID(rootID))
	if err != nil {
		t.Fatal(err)
	}
	return page
}

func readManifest(t *testing.T, dir string) Manifest {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func assertNoStaging(t *testing.T, out string) {
	t.Helper()
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), stagingPrefix) {
			t.Fatalf("staging/quarantine left behind: %s", e.Name())
		}
	}
}

func TestExportWritesCompleteDirectoryAndManifest(t *testing.T) {
	srv, _ := assetServer(t)
	_, svc := exportFixture(t, srv.URL)
	svc.HTTP = srv.Client()
	page := fetchFixturePage(t, svc)
	out := filepath.Join(t.TempDir(), "nested", "exports")
	res, err := svc.Export(context.Background(), page, ExportOptions{OutDir: out, Format: FormatBoth, Selected: SelectAll(page.Assets, map[AssetType]bool{AssetImage: true, AssetVideo: true}), Concurrency: 2})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(out, "minha-pagina")
	if res.Dir != dir || res.Downloaded != 1 || res.Failed != 1 || res.Skipped != 0 || res.Discovered != 2 {
		t.Fatalf("result %+v", res)
	}
	for _, name := range []string{"page.md", "page.html", "styles.css", "page.json", "manifest.json", "assets/images/001-foto.png"} {
		if info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(name))); err != nil || !info.Mode().IsRegular() {
			t.Errorf("%s: %v", name, err)
		}
	}
	md, _ := os.ReadFile(filepath.Join(dir, "page.md"))
	if !strings.Contains(string(md), "![Foto](assets/images/001-foto.png)") || !strings.Contains(string(md), srv.URL+"/login.mp4") {
		t.Fatalf("markdown must reference local and remote fallback assets:\n%s", md)
	}
	m := readManifest(t, dir)
	if m.SchemaVersion != 1 || m.Generator.Name != GeneratorName || m.Generator.Version != "9.9.9" || m.Format != FormatBoth || m.PageID != CompactID(rootID) || m.ExportedAt != "2026-01-02T03:04:05.006Z" {
		t.Fatalf("manifest header %+v", m)
	}
	if m.Source == nil || !strings.Contains(*m.Source, "notion.site") || m.Files.Markdown == nil || *m.Files.HTML != "page.html" || m.Files.RecordMap != "page.json" {
		t.Fatalf("manifest files/source %+v", m)
	}
	if m.Assets.Downloaded != 1 || m.Assets.Failed != 1 || m.Assets.Items[0].Status != StatusDownloaded || *m.Assets.Items[0].Path != "assets/images/001-foto.png" || m.Assets.Items[1].Error == nil {
		t.Fatalf("manifest assets %+v", m.Assets)
	}
	assertNoStaging(t, out)

	// page.json round-trips through --from-json and selects the same page.
	again, err := svc.LoadRecordMap(filepath.Join(dir, "page.json"))
	if err != nil || again.PageID != page.PageID || len(again.Assets) != 2 {
		t.Fatalf("reload: %+v %v", again, err)
	}
}

func TestExportFormatsAndExistingDestination(t *testing.T) {
	_, svc := exportFixture(t, "https://cdn.invalid")
	page := fetchFixturePage(t, svc)
	out := t.TempDir()
	res, err := svc.Export(context.Background(), page, ExportOptions{OutDir: out, Format: FormatMarkdown})
	if err != nil {
		t.Fatal(err)
	}
	dir := res.Dir
	if _, err := os.Stat(filepath.Join(dir, "page.html")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("html written for --format md")
	}
	if m := readManifest(t, dir); m.Files.HTML != nil || m.Files.Styles != nil || m.Assets.Skipped != 2 {
		t.Fatalf("manifest %+v", m)
	}
	before, _ := os.ReadFile(filepath.Join(dir, "page.md"))
	if err := svc.CheckDestination(out, page, false); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("CheckDestination: %v", err)
	}
	if _, err := svc.Export(context.Background(), page, ExportOptions{OutDir: out, Format: FormatHTML}); !errors.Is(err, ErrDestinationExists) {
		t.Fatalf("no-clobber: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "page.md"))
	if string(before) != string(after) {
		t.Fatal("existing export modified without --force")
	}
	assertNoStaging(t, out)

	// --force replaces a previous export of the same page.
	if err := os.WriteFile(filepath.Join(dir, "stale.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Export(context.Background(), page, ExportOptions{OutDir: out, Format: FormatHTML, Force: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "stale.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("previous export not replaced")
	}
	if _, err := os.Stat(filepath.Join(dir, "page.html")); err != nil {
		t.Fatal(err)
	}
	assertNoStaging(t, out)
}

func TestForceRefusesForeignDestinations(t *testing.T) {
	_, svc := exportFixture(t, "https://cdn.invalid")
	page := fetchFixturePage(t, svc)
	cases := map[string]func(dir string){
		"no manifest": func(dir string) {
			os.MkdirAll(dir, 0o755)
			os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)
		},
		"other generator": func(dir string) {
			writeJSON(t, dir, map[string]any{"generator": map[string]any{"name": "other"}, "pageId": CompactID(rootID)})
		},
		"other page": func(dir string) {
			writeJSON(t, dir, map[string]any{"generator": map[string]any{"name": GeneratorName}, "pageId": "ffffffffffffffffffffffffffffffff"})
		},
		"invalid json": func(dir string) {
			os.MkdirAll(dir, 0o755)
			os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{"), 0o644)
		},
		"regular file": func(dir string) { os.WriteFile(dir, []byte("x"), 0o644) },
		"symlink": func(dir string) {
			target := t.TempDir()
			writeJSON(t, target, map[string]any{"generator": map[string]any{"name": GeneratorName}, "pageId": CompactID(rootID)})
			if err := os.Symlink(target, dir); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			out := t.TempDir()
			dir := filepath.Join(out, page.Slug)
			setup(dir)
			before := snapshot(t, out)
			_, err := svc.Export(context.Background(), page, ExportOptions{OutDir: out, Format: FormatBoth, Force: true})
			if !errors.Is(err, ErrForeignDestination) {
				t.Fatalf("err = %v", err)
			}
			if after := snapshot(t, out); after != before {
				t.Fatalf("destination changed:\n%s\n%s", before, after)
			}
		})
	}
	// The legacy Deno exporter's manifests are accepted for the same page.
	out := t.TempDir()
	writeJSON(t, filepath.Join(out, page.Slug), map[string]any{"generator": map[string]any{"name": LegacyGeneratorName}, "pageId": CompactID(rootID)})
	if _, err := svc.Export(context.Background(), page, ExportOptions{OutDir: out, Format: FormatBoth, Force: true}); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, dir string, v any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(v)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, _ := os.Lstat(path)
		b.WriteString(rel + " " + info.Mode().String())
		if info.Mode().IsRegular() {
			data, _ := os.ReadFile(path)
			b.WriteString(" " + string(data))
		}
		b.WriteString("\n")
		return nil
	})
	return b.String()
}

// failingOps wraps a safety root and fails the rename that installs staging.
type failingOps struct {
	*safefs.Root
	failInstall bool
	failRestore bool
	staging     string
}

func (f *failingOps) Rename(oldname, newname string) error {
	if f.failInstall && oldname == f.staging {
		return errors.New("synthetic install failure")
	}
	if f.failRestore && strings.HasSuffix(oldname, ".old") {
		return errors.New("synthetic restore failure")
	}
	return f.Root.Rename(oldname, newname)
}

func TestForcePublicationRollsBackWhenInstallFails(t *testing.T) {
	out := t.TempDir()
	writeJSON(t, filepath.Join(out, "page"), map[string]any{"generator": map[string]any{"name": GeneratorName}, "pageId": CompactID(rootID)})
	os.WriteFile(filepath.Join(out, "page", "old.txt"), []byte("previous"), 0o644)
	staging := stagingPrefix + "page-x.tmp"
	os.MkdirAll(filepath.Join(out, staging), 0o755)
	root, err := safefs.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ops := &failingOps{Root: root, failInstall: true, staging: staging}
	_, err = publication{root: ops, staging: staging, target: "page", pageID: rootID, force: true}.publish()
	if err == nil || !strings.Contains(err.Error(), "export anterior restaurado") {
		t.Fatalf("err = %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(out, "page", "old.txt")); err != nil || string(data) != "previous" {
		t.Fatal("previous export not restored")
	}
	// A failing restore reports where the previous export was kept.
	ops.failRestore = true
	_, err = publication{root: ops, staging: staging, target: "page", pageID: rootID, force: true}.publish()
	if err == nil || !strings.Contains(err.Error(), ".old") {
		t.Fatalf("restore failure must name the quarantine: %v", err)
	}
}

func TestPublishWithoutForceNeverReplaces(t *testing.T) {
	out := t.TempDir()
	os.MkdirAll(filepath.Join(out, "page"), 0o755)
	os.WriteFile(filepath.Join(out, "page", "keep"), []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(out, "stage"), 0o755)
	root, _ := safefs.Open(out)
	defer root.Close()
	if _, err := (publication{root: root, staging: "stage", target: "page", pageID: rootID}).publish(); !errors.Is(err, ErrDestinationExists) {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "page", "keep")); err != nil {
		t.Fatal("existing content lost")
	}
}

func TestExportCancellationRemovesStaging(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-block:
		}
	}))
	defer srv.Close()
	defer close(block)
	_, svc := exportFixture(t, srv.URL)
	svc.HTTP = srv.Client()
	page := fetchFixturePage(t, svc)
	out := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	_, err := svc.Export(ctx, page, ExportOptions{OutDir: out, Format: FormatBoth, Selected: SelectAll(page.Assets, map[AssetType]bool{AssetImage: true}), Concurrency: 1,
		OnDownload: func(ev DownloadEvent) {
			if ev.Started {
				go func() { time.Sleep(20 * time.Millisecond); cancel() }()
			}
		}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
	entries, _ := os.ReadDir(out)
	if len(entries) != 0 {
		t.Fatalf("cancelled export left %v", entries)
	}
}

func TestExportRejectsUnsafeOutput(t *testing.T) {
	_, svc := exportFixture(t, "https://cdn.invalid")
	page := fetchFixturePage(t, svc)
	base := t.TempDir()
	file := filepath.Join(base, "file")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := svc.Export(context.Background(), page, ExportOptions{OutDir: filepath.Join(file, "sub"), Format: FormatBoth}); err == nil {
		t.Fatal("output below a regular file accepted")
	}
	if err := svc.CheckDestination(file, page, false); err == nil {
		t.Fatal("regular file accepted as output directory")
	}
	if _, err := svc.Export(context.Background(), page, ExportOptions{OutDir: base, Format: "pdf"}); err == nil {
		t.Fatal("invalid format accepted")
	}
	if _, err := svc.Fetch(context.Background(), "not an id"); !errors.Is(err, ErrInvalidPageID) {
		t.Fatal(err)
	}
	failing := Service{Source: fakeSource{err: errors.New("boom")}}
	if _, err := failing.Fetch(context.Background(), CompactID(rootID)); err == nil {
		t.Fatal("source error swallowed")
	}
}

func TestStaleStagingIsReportedNotDeleted(t *testing.T) {
	_, svc := exportFixture(t, "https://cdn.invalid")
	var messages []string
	svc.Progress = func(m string) { messages = append(messages, m) }
	page := fetchFixturePage(t, svc)
	out := t.TempDir()
	stale := filepath.Join(out, stagingPrefix+page.Slug+"-dead.tmp")
	os.MkdirAll(stale, 0o755)
	if _, err := svc.Export(context.Background(), page, ExportOptions{OutDir: out, Format: FormatMarkdown}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatal("stale staging must not be deleted automatically")
	}
	if !strings.Contains(strings.Join(messages, "\n"), "export interrompido") {
		t.Fatalf("messages: %v", messages)
	}
}

func TestValidateExportName(t *testing.T) {
	for _, ok := range []string{"backup", "pagina-2", "v1.0_final"} {
		if err := ValidateExportName(ok); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	for _, bad := range []string{"", ".x", "x.", "..", "a/b", `a\\b`, "á", "a b", strings.Repeat("a", 81), "-a"} {
		if ValidateExportName(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
