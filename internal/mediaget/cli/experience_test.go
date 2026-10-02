package mediacli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

func TestConfigurationRefererRefreshesMetadataAndSizes(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	b := &backend{info: mediaget.Info{Title: "Synthetic", Size: mediaget.FormatSize{Bytes: 4096}}}
	var log bytes.Buffer
	// Audio, add config, search Referer, pick it, value, reselect audio, continue.
	input := "2\n4\nref\n1\nhttps://origin.invalid/page\n2\n1\n\ns\n"
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid/media"}, core.IO{In: strings.NewReader(input), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 || b.last.Source.Referer != "https://origin.invalid/page" {
		t.Fatalf("%+v", b.last)
	}
	old, updated := 0, 0
	for _, src := range b.sources {
		if src.Referer == "" {
			old++
		} else {
			updated++
		}
	}
	if old < 1 || updated < 1 || old > 8 || updated > 8 {
		t.Fatalf("queries before=%d after=%d", old, updated)
	}
	if strings.Contains(log.String(), "https://origin.invalid/page") {
		t.Fatal("Referer leaked into wrapper output")
	}
}
func TestFragmentConfigurationDoesNotRepeatQueriesOrSelections(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	b := &backend{info: mediaget.Info{Title: "Synthetic"}}
	var log bytes.Buffer
	input := "2\n4\nfrag\n1\n9\n4\n1\n\ns\n"
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid/media"}, core.IO{In: strings.NewReader(input), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	if b.last.Source.ConcurrentFragments != 4 || b.calls < 1 || b.calls > 8 {
		t.Fatalf("%+v calls=%d", b.last, b.calls)
	}
	if strings.Count(log.String(), "O que deseja baixar?") != 1 {
		t.Fatal("configuration repeated type selection")
	}
}
func TestNativeSelectorSearchAndTerminalSanitization(t *testing.T) {
	m := selectionModel{choices: []string{"Referer", "Fragmentos paralelos", "Voltar"}, query: []rune("REF")}
	matches := m.matches()
	if len(matches) != 1 || matches[0] != 0 {
		t.Fatal(matches)
	}
	m.query = []rune("ausente")
	if len(m.matches()) != 0 {
		t.Fatal("nonmatching option returned")
	}
	got := fitLine("abc\x1b\n\r\u202edef", 6)
	if got != "abcdef" {
		t.Fatalf("unsafe/truncated line %q", got)
	}
	if got := fitLine("界界界", 4); got != "界界" {
		t.Fatalf("wide line %q", got)
	}
}
func TestTTYProgressReplacesLineAndDistinguishesEstimates(t *testing.T) {
	var output bytes.Buffer
	inv := &core.Invocation{IO: core.IO{Err: &output}, Terminal: core.Terminal{StderrTTY: true, Width: 100}}
	r := newDownloadRenderer(inv)
	r.current = mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 1024, Total: 2048, Speed: 1024, ETA: 2, Estimated: true}
	r.render(true)
	text := output.String()
	for _, want := range []string{"\r\x1b[2K", "█", "░", "50%", "1.0 KiB / ≈ 2.0 KiB", "1.0 KiB/s", "2s"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q: %q", want, text)
		}
	}
	if strings.Contains(text, "\n") {
		t.Fatal("progress grew terminal history")
	}
	output.Reset()
	r.current = mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 1024}
	r.render(true)
	if !strings.Contains(output.String(), "total desconhecido") || strings.Contains(output.String(), "%") {
		t.Fatal(output.String())
	}
	output.Reset()
	r.current = mediaget.Progress{Stage: mediaget.Processing}
	r.render(true)
	if !strings.Contains(output.String(), "Processando") || strings.Contains(output.String(), "100%") {
		t.Fatal(output.String())
	}
}

type failedOutput struct{ writes int }

func (w *failedOutput) Write(p []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		return len(p), nil
	}
	return 0, io.ErrClosedPipe
}

type cancelBackend struct {
	backend
	stopped chan struct{}
}

func (b *cancelBackend) Download(ctx context.Context, _ mediaget.Request, _ string, notify func(mediaget.Progress)) error {
	defer close(b.stopped)
	notify(mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 1, Total: 100})
	<-ctx.Done()
	return ctx.Err()
}
func TestProgressOutputFailureCancelsBackend(t *testing.T) {
	b := &cancelBackend{stopped: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	inv := &core.Invocation{Context: ctx, IO: core.IO{Err: &failedOutput{}}}
	req := mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid"}, Selection: mediaget.Selection{Kind: mediaget.Audio}, OutputDir: t.TempDir(), Name: "Synthetic"}
	_, err := downloadWithProgress(inv, mediaget.Service{Backend: b}, req, newDownloadRenderer(inv))
	if !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	select {
	case <-b.stopped:
	default:
		t.Fatal("backend survived failed progress output")
	}
}

type refererRequiredBackend struct{ backend }

func (b *refererRequiredBackend) Inspect(ctx context.Context, src mediaget.Source, sel mediaget.Selection) (mediaget.Info, error) {
	if src.Referer == "" {
		return mediaget.Info{}, errors.New("synthetic site requires Referer")
	}
	return b.backend.Inspect(ctx, src, sel)
}
func TestFailedInitialLookupCanRecoverWithReferer(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	b := &refererRequiredBackend{backend: backend{info: mediaget.Info{Title: "Synthetic"}}}
	a, err := New(mediaget.Service{Backend: b}, core.ProductMetadata{Version: "0.2.0"})
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	input := "1\nref\n1\nhttps://origin.invalid\n2\n1\n\ns\n"
	err = a.Run(context.Background(), []string{"https://example.invalid"}, core.IO{In: strings.NewReader(input), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 || b.last.Source.Referer != "https://origin.invalid" {
		t.Fatalf("%+v", b.last)
	}
	if strings.Contains(log.String(), "https://origin.invalid") {
		t.Fatal("Referer leaked into log")
	}
}

func TestProgressFitsNarrowTerminal(t *testing.T) {
	var output bytes.Buffer
	inv := &core.Invocation{IO: core.IO{Err: &output}, Terminal: core.Terminal{StderrTTY: true, Width: 18}}
	r := newDownloadRenderer(inv)
	r.current = mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 1024, Total: 2048}
	r.render(true)
	line := strings.TrimPrefix(output.String(), "\r\x1b[2K")
	if utf8.RuneCountInString(line) > 17 || strings.Contains(line, "\n") {
		t.Fatalf("narrow terminal wrapped: %q", line)
	}
}

func TestSelectedPreviewSeedsBarBeforeFirstDownloadEvent(t *testing.T) {
	var output bytes.Buffer
	inv := &core.Invocation{IO: core.IO{Err: &output}, Terminal: core.Terminal{StderrTTY: true, Width: 100}}
	r := newDownloadRenderer(inv)
	r.setEstimate(transferEstimate{bytes: 4096, known: true})
	r.accept(mediaget.Progress{Stage: mediaget.Transferring})
	r.render(true)
	if !strings.Contains(output.String(), "░") || !strings.Contains(output.String(), "0 B / ≈ 4.0 KiB") || strings.Contains(output.String(), "desconhecido") {
		t.Fatal(output.String())
	}
	r.accept(mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 1024})
	if r.current.Total != 4096 || !r.current.Estimated {
		t.Fatal(r.current)
	}
}
func TestVideoAndAudioProgressAggregatesWithoutDoubleCounting(t *testing.T) {
	r := newDownloadRenderer(&core.Invocation{})
	r.setEstimate(transferEstimate{bytes: 5000, known: true, parts: []mediaget.FormatSize{{Bytes: 4000, StreamID: "video"}, {Bytes: 1000, StreamID: "audio"}}})
	r.accept(mediaget.Progress{Stage: mediaget.Transferring, StreamID: "video", Downloaded: 4000, Total: 4000})
	if r.current.Total != 5000 || r.current.Downloaded != 4000 || !r.current.Estimated {
		t.Fatal(r.current)
	}
	r.accept(mediaget.Progress{Stage: mediaget.Transferring, StreamID: "audio", Downloaded: 500, Total: 1000, Speed: 250})
	if r.current.Total != 5000 || r.current.Downloaded != 4500 || r.current.Estimated || r.current.ETA != 2 {
		t.Fatal(r.current)
	}
	r.accept(mediaget.Progress{Stage: mediaget.Transferring, StreamID: "audio", Downloaded: 500, Total: 1000})
	if r.current.Downloaded != 4500 {
		t.Fatal("duplicate progress added bytes twice")
	}
	r.accept(mediaget.Progress{Stage: mediaget.Transferring, StreamID: "audio", Downloaded: 1000, Total: 1000})
	if r.current.Downloaded != 5000 || r.current.Total != 5000 {
		t.Fatal(r.current)
	}
}
func TestUnknownActualTotalsKeepChosenForecastAcrossStreams(t *testing.T) {
	r := newDownloadRenderer(&core.Invocation{})
	r.setEstimate(transferEstimate{bytes: 4096, known: true})
	r.accept(mediaget.Progress{Stage: mediaget.Transferring, StreamID: "video", Downloaded: 1024})
	r.accept(mediaget.Progress{Stage: mediaget.Transferring, StreamID: "audio", Downloaded: 512})
	if r.current.Downloaded != 1536 || r.current.Total != 4096 || !r.current.Estimated {
		t.Fatal(r.current)
	}
}

func TestCancellationKeepsLastProgressVisible(t *testing.T) {
	var output bytes.Buffer
	inv := &core.Invocation{Context: context.Background(), IO: core.IO{Err: &output}, Terminal: core.Terminal{StderrTTY: true, Width: 100}}
	b := &canceledProgressBackend{}
	req := mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid"}, Selection: mediaget.Selection{Kind: mediaget.Audio}, OutputDir: t.TempDir(), Name: "Synthetic"}
	r := newDownloadRenderer(inv)
	r.setEstimate(transferEstimate{bytes: 4096, known: true})
	_, err := downloadWithProgress(inv, mediaget.Service{Backend: b}, req, r)
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !strings.HasSuffix(output.String(), "\n") || strings.HasSuffix(output.String(), "\r\x1b[2K") {
		t.Fatal("last progress line erased on cancellation")
	}
	if !strings.Contains(output.String(), "1.0 KiB / ≈ 4.0 KiB") {
		t.Fatal(output.String())
	}
}

type canceledProgressBackend struct{ backend }

func (b *canceledProgressBackend) Download(_ context.Context, _ mediaget.Request, _ string, notify func(mediaget.Progress)) error {
	notify(mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 1024, StreamID: "synthetic"})
	return context.Canceled
}
