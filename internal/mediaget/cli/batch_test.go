package mediacli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type batchCLIBackend struct {
	mu                     sync.Mutex
	inspections, downloads int
	requests               []mediaget.Request
	title                  string
	bad                    string
	failDownload           bool
}

func (b *batchCLIBackend) Check(context.Context, mediaget.Selection) error { return nil }
func (b *batchCLIBackend) Inspect(_ context.Context, src mediaget.Source, _ mediaget.Selection) (mediaget.Info, error) {
	b.mu.Lock()
	b.inspections++
	b.mu.Unlock()
	if src.URL == b.bad {
		return mediaget.Info{}, errors.New("synthetic inspection failure")
	}
	return mediaget.Info{Title: b.title, Size: mediaget.FormatSize{Bytes: 100}, Tracks: []mediaget.Track{{Lang: "en", Auto: true}}}, nil
}
func (b *batchCLIBackend) Download(_ context.Context, r mediaget.Request, dir string, p func(mediaget.Progress)) error {
	b.mu.Lock()
	b.downloads++
	b.requests = append(b.requests, r)
	b.mu.Unlock()
	if b.failDownload {
		return errors.New("synthetic transfer failure")
	}
	return os.WriteFile(filepath.Join(dir, "media.mp4"), []byte("synthetic"), 0600)
}
func batchApp(t *testing.T, b *batchCLIBackend) *core.CompiledApp {
	t.Helper()
	a, err := New(mediaget.Service{Backend: b}, core.ProductMetadata{Version: "0.3.1"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func manifestFile(t *testing.T, data string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestBatchStaticAndOfflineValidationIgnoreEnvironment(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv(DownloadEnv, "")
	t.Setenv(FragmentsEnv, "invalid")
	b := &batchCLIBackend{}
	a := batchApp(t, b)
	p := manifestFile(t, `{"schemaVersion":1,"items":[{"url":"https://example.invalid","title":"Synthetic"}]}`)
	for _, args := range [][]string{{"batch", "schema"}, {"batch", "example"}, {"batch", "--help"}, {"batch", "validate", p}} {
		var out bytes.Buffer
		if err := a.Run(t.Context(), args, core.IO{Out: &out}); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if out.Len() == 0 {
			t.Fatal(args)
		}
	}
	if b.inspections != 0 || b.downloads != 0 {
		t.Fatal(b)
	}
}
func TestBatchFlagsOverrideJSONAndInvalidEnvironmentIsUnused(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(DownloadEnv, "")
	t.Setenv(FragmentsEnv, "bad")
	dirJSON, _ := json.Marshal(dir)
	p := manifestFile(t, `{"schemaVersion":1,"defaults":{"outputDir":`+string(dirJSON)+`,"concurrentFragments":25},"items":[{"url":"https://example.invalid/a","name":"first.mp4","origin":"https://example.invalid"},{"url":"https://example.invalid/b","title":"second","options":{"concurrentFragments":16}}]}`)
	b := &batchCLIBackend{title: "metadata"}
	a := batchApp(t, b)
	var out, log bytes.Buffer
	err := a.Run(t.Context(), []string{"batch", p, "--jobs", "2", "--concurrent-fragments", "8", "--yes"}, core.IO{Out: &out, Err: &log})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 2 || strings.Count(out.String(), ".mp4\n") != 2 {
		t.Fatal(out.String(), b.downloads)
	}
	for _, r := range b.requests {
		if r.Source.ConcurrentFragments != 8 || r.OutputDir != dir {
			t.Fatalf("%+v", r)
		}
		if r.Name == "first" && r.Selection.VideoFormat != "mp4" {
			t.Fatal(r)
		}
	}
	if !strings.Contains(log.String(), "Formato de saída: MP4 (.mp4)") || !strings.Contains(log.String(), "Origin: definido") || strings.Contains(log.String(), "https://") {
		t.Fatal(log.String())
	}
}
func TestBatchPreparationFailureNeverStartsDownloads(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	t.Setenv(FragmentsEnv, "4")
	p := manifestFile(t, `{"schemaVersion":1,"items":[{"url":"https://example.invalid/a","name":"first"},{"url":"https://example.invalid/b","name":"second"}]}`)
	b := &batchCLIBackend{bad: "https://example.invalid/b"}
	a := batchApp(t, b)
	if err := a.Run(t.Context(), []string{"batch", p, "--yes"}, core.IO{}); err == nil || b.downloads != 0 {
		t.Fatal(err, b.downloads)
	}
	b = &batchCLIBackend{}
	a = batchApp(t, b)
	if err := a.Run(t.Context(), []string{"batch", p, "--jobs", "9", "--yes"}, core.IO{}); core.ExitCode(err) != 2 || b.inspections != 0 {
		t.Fatal(err, b.inspections)
	}
}

func TestBatchInvalidDestinationFailsBeforeInspection(t *testing.T) {
	t.Setenv(FragmentsEnv, "4")
	file := filepath.Join(t.TempDir(), "regular-file")
	if err := os.WriteFile(file, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	invalid, _ := json.Marshal(file)
	p := manifestFile(t, `{"schemaVersion":1,"defaults":{"outputDir":`+string(invalid)+`},"items":[{"url":"https://example.invalid","name":"first"}]}`)
	b := &batchCLIBackend{}
	a := batchApp(t, b)
	if err := a.Run(t.Context(), []string{"batch", p, "--yes"}, core.IO{}); err == nil || b.inspections != 0 || b.downloads != 0 {
		t.Fatal(err, b.inspections, b.downloads)
	}
}

func TestBatchCreatesSharedNestedDestinationOnlyAfterConfirmation(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "nested")
	t.Setenv(FragmentsEnv, "4")
	p := manifestFile(t, `{"schemaVersion":1,"items":[{"url":"https://example.invalid/a","name":"first"},{"url":"https://example.invalid/b","name":"second"}]}`)
	b := &batchCLIBackend{}
	a := batchApp(t, b)
	if err := a.Run(t.Context(), []string{"batch", p, "--output-dir", dir}, core.IO{In: strings.NewReader("5\n"), Terminal: core.Terminal{StdinTTY: true}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) || b.downloads != 0 {
		t.Fatal("cancel created destination", err, b.downloads)
	}
	if err := a.Run(t.Context(), []string{"batch", p, "--output-dir", dir, "--jobs", "2", "--yes"}, core.IO{}); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 2 || b.downloads != 2 {
		t.Fatal(err, files, b.downloads)
	}
}

func TestBatchEditedMissingDestinationReusesMetadata(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	t.Setenv(FragmentsEnv, "4")
	dir := filepath.Join(t.TempDir(), "new-destination")
	p := manifestFile(t, `{"schemaVersion":1,"items":[{"url":"https://example.invalid","name":"first"}]}`)
	b := &batchCLIBackend{}
	a := batchApp(t, b)
	input := "2\n1\n2\n" + dir + "\n1\n"
	if err := a.Run(t.Context(), []string{"batch", p}, core.IO{In: strings.NewReader(input), Terminal: core.Terminal{StdinTTY: true}}); err != nil {
		t.Fatal(err)
	}
	if b.inspections != 1 || b.downloads != 1 || b.requests[0].OutputDir != dir {
		t.Fatal(b.inspections, b.downloads, b.requests)
	}
}
func TestBatchMissingNamePromptAndCancel(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	t.Setenv(FragmentsEnv, "4")
	p := manifestFile(t, `{"schemaVersion":1,"items":[{"url":"https://example.invalid"}]}`)
	b := &batchCLIBackend{}
	a := batchApp(t, b)
	if err := a.Run(t.Context(), []string{"batch", p, "--yes"}, core.IO{}); err == nil || b.downloads != 0 {
		t.Fatal(err, b.downloads)
	}
	var out bytes.Buffer
	if err := a.Run(t.Context(), []string{"batch", p}, core.IO{In: strings.NewReader("Synthetic name\n1\n"), Out: &out, Terminal: core.Terminal{StdinTTY: true}}); err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 || b.requests[0].Name != "Synthetic name" {
		t.Fatal(b.requests)
	}
	b = &batchCLIBackend{title: "Metadata"}
	a = batchApp(t, b)
	if err := a.Run(t.Context(), []string{"batch", p}, core.IO{In: strings.NewReader("5\n"), Terminal: core.Terminal{StdinTTY: true}}); err != nil || b.downloads != 0 {
		t.Fatal(err, b.downloads)
	}
}
func TestBatchRepairExcludeAndNameEditReusesMetadata(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	t.Setenv(FragmentsEnv, "4")
	p := manifestFile(t, `{"schemaVersion":1,"items":[{"url":"https://example.invalid/a","name":"first"},{"url":"https://example.invalid/b","name":"second"}]}`)
	b := &batchCLIBackend{bad: "https://example.invalid/b"}
	a := batchApp(t, b)
	// Edit first name, then exclude the item whose preparation failed, download.
	input := "2\n1\n1\nnew name\n2\n2\n9\n1\n"
	if err := a.Run(t.Context(), []string{"batch", p}, core.IO{In: strings.NewReader(input), Terminal: core.Terminal{StdinTTY: true}}); err != nil {
		t.Fatal(err)
	}
	if b.inspections != 2 || b.downloads != 1 || b.requests[0].Name != "new name" {
		t.Fatal(b.inspections, b.downloads, b.requests)
	}
}
func TestOutputFormatExtensions(t *testing.T) {
	for _, tc := range []struct {
		sel mediaget.Selection
		ext string
	}{
		{mediaget.Selection{Kind: mediaget.Subtitle, SubtitleFormat: "txt"}, ".txt"},
		{mediaget.Selection{Kind: mediaget.Subtitle}, ".srt"},
		{mediaget.Selection{Kind: mediaget.Video, VideoFormat: "mp4"}, ".mp4"},
		{mediaget.Selection{Kind: mediaget.Audio}, ".m4a"},
		{mediaget.Selection{Kind: mediaget.Video}, ""},
	} {
		label, ext := outputFormat(tc.sel)
		if ext != tc.ext || tc.ext != "" && !strings.Contains(label, "("+tc.ext+")") {
			t.Fatal(label, ext)
		}
	}
}

func TestBatchNamesRemainDistinctAfterSafeNameByteLimit(t *testing.T) {
	base := strings.Repeat("á", 90)
	entries := []batchEntry{{req: mediaget.Request{Name: base}}, {req: mediaget.Request{Name: base}}, {req: mediaget.Request{Name: base + " (1)"}}}
	uniqueBatchNames(entries)
	seen := map[string]bool{}
	for _, e := range entries {
		name := mediaget.SafeName(e.req.Name)
		if seen[name] {
			t.Fatal("duplicate", name)
		}
		seen[name] = true
	}
}

type shortBatchWriter struct{}

func (shortBatchWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }
func TestBatchShortSummaryWriteBlocksDownloads(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	t.Setenv(FragmentsEnv, "4")
	p := manifestFile(t, `{"schemaVersion":1,"items":[{"url":"https://example.invalid","name":"Synthetic"}]}`)
	b := &batchCLIBackend{}
	a := batchApp(t, b)
	if err := a.Run(t.Context(), []string{"batch", p, "--yes"}, core.IO{Err: shortBatchWriter{}}); !errors.Is(err, io.ErrShortWrite) || b.downloads != 0 {
		t.Fatal(err, b.downloads)
	}
}

func TestBatchDirectAndAllVideoFormatEditsPreserveQuality(t *testing.T) {
	for _, input := range []string{"2\n1\n11\n2\n1\n", "3\n2\n1\n"} {
		t.Run(strings.ReplaceAll(input, "\n", "-"), func(t *testing.T) {
			t.Setenv(DownloadEnv, t.TempDir())
			t.Setenv(FragmentsEnv, "4")
			p := manifestFile(t, `{"schemaVersion":1,"defaults":{"quality":"360"},"items":[{"url":"https://example.invalid/a","name":"video"}]}`)
			b := &batchCLIBackend{}
			a := batchApp(t, b)
			var log bytes.Buffer
			err := a.Run(t.Context(), []string{"batch", p}, core.IO{In: strings.NewReader(input), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
			if err != nil {
				t.Fatal(err, log.String())
			}
			if len(b.requests) != 1 || b.requests[0].Selection.VideoFormat != "mp4" || b.requests[0].Selection.Height != 360 {
				t.Fatal(b.requests)
			}
			if !strings.Contains(log.String(), "MP4") {
				t.Fatal(log.String())
			}
		})
	}
}
