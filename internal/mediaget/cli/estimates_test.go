package mediacli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type previewBackend struct {
	backend
	mu           sync.Mutex
	active, peak int
	calls        map[mediaget.Selection]int
	started      chan struct{}
	stopped      chan struct{}
	release      chan struct{}
}

func (b *previewBackend) Inspect(ctx context.Context, _ mediaget.Source, sel mediaget.Selection) (mediaget.Info, error) {
	b.mu.Lock()
	if b.calls == nil {
		b.calls = make(map[mediaget.Selection]int)
	}
	b.calls[sel]++
	b.active++
	b.peak = max(b.peak, b.active)
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		b.active--
		b.mu.Unlock()
		if b.stopped != nil {
			b.stopped <- struct{}{}
		}
	}()
	if b.started != nil {
		b.started <- struct{}{}
	}
	if b.release != nil {
		select {
		case <-ctx.Done():
			return mediaget.Info{}, ctx.Err()
		case <-b.release:
		}
	}
	if sel.Kind == "" {
		return mediaget.Info{Title: "Preview", Heights: []int{360, 720, 1080}}, nil
	}
	if sel.Kind == mediaget.Audio {
		return mediaget.Info{Size: mediaget.FormatSize{Bytes: 4096}}, nil
	}
	if sel.Height == 480 {
		return mediaget.Info{}, context.DeadlineExceeded
	}
	if sel.Height == 360 {
		return mediaget.Info{Parts: []mediaget.FormatSize{{Bytes: 1024}, {}}}, nil
	}
	return mediaget.Info{Parts: []mediaget.FormatSize{{Bytes: float64(8192 + sel.Height)}, {Bytes: 4096}}}, nil
}

func (b *previewBackend) Download(ctx context.Context, req mediaget.Request, work string, notify func(mediaget.Progress)) error {
	if notify != nil {
		notify(mediaget.Progress{Stage: mediaget.Transferring})
		notify(mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 1024, Total: 2048, Speed: 1024, ETA: 2})
		notify(mediaget.Progress{Stage: mediaget.Processing})
	}
	return b.backend.Download(ctx, req, work, notify)
}

func TestPreviewMenusShowSizesBeforeSelectionAndReuseResults(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	b := &previewBackend{}
	a, err := New(mediaget.Service{Backend: b}, core.ProductMetadata{Version: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	var out, log bytes.Buffer
	// Video, return to type menu, audio, continue, default name, confirm.
	err = a.Run(context.Background(), []string{"https://example.invalid", "--referer", ""}, core.IO{
		In: strings.NewReader("1\n0\n2\n1\n\ns\n"), Out: &out, Err: &log, Terminal: core.Terminal{StdinTTY: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	text := log.String()
	for _, line := range []string{
		"Vídeo com áudio (melhor qualidade) — ≈ 12.0 KiB",
		"Somente áudio (M4A) — ≈ 4.0 KiB",
		"Até 720p — ≈ 12.7 KiB", "Até 480p — tamanho indisponível", "Até 360p — tamanho indisponível",
		"Calculando tamanhos estimados: 2/2", "Calculando tamanhos estimados: 5/5",
		"Transferência atual: 50% (1.0 KiB / 2.0 KiB) | 1.0 KiB/s | restante: 2s",
		"Processando mídia (conversão/mesclagem)...", "Verificando e salvando arquivo...",
	} {
		if !strings.Contains(text, line) {
			t.Fatalf("missing %q in %s", line, text)
		}
	}
	if strings.Index(text, "Somente áudio (M4A) — ≈") > strings.Index(text, "O que deseja baixar?") ||
		strings.Index(text, "Até 720p — ≈") > strings.Index(text, "Qualidade do vídeo") {
		t.Fatal("preview was printed after selection")
	}
	for selection, calls := range b.calls {
		if calls != 1 {
			t.Fatalf("repeated query for %+v: %d", selection, calls)
		}
	}
	if b.downloads != 1 || b.last.Selection.Kind != mediaget.Audio || !strings.HasSuffix(strings.TrimSpace(out.String()), ".m4a") {
		t.Fatal("selected audio was not delivered")
	}
}

func TestPreviewQueriesRunInParallelWithBoundedConcurrency(t *testing.T) {
	b := &previewBackend{started: make(chan struct{}, 8), release: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var log bytes.Buffer
	inv := &core.Invocation{Context: ctx, IO: core.IO{Err: &log}}
	selections := []mediaget.Selection{{Kind: mediaget.Audio}}
	for _, height := range []int{0, 2160, 1080, 720, 480, 360} {
		selections = append(selections, mediaget.Selection{Kind: mediaget.Video, Height: height})
	}
	cache := make(map[mediaget.Selection]transferEstimate)
	done := make(chan error, 1)
	go func() {
		done <- estimateOptions(inv, mediaget.Service{Backend: b}, mediaget.Source{URL: "https://example.invalid"}, selections, cache)
	}()
	for range estimateConcurrency {
		select {
		case <-b.started:
		case <-ctx.Done():
			t.Fatal("queries did not overlap")
		}
	}
	close(b.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if b.peak != estimateConcurrency || len(cache) != len(selections) {
		t.Fatalf("peak=%d estimates=%d", b.peak, len(cache))
	}
	if cache[mediaget.Selection{Kind: mediaget.Video, Height: 480}].known {
		t.Fatal("timeout became a valid estimate")
	}
	if !strings.Contains(log.String(), "7/7") || strings.Contains(log.String(), "\x1b") {
		t.Fatalf("redirected loading status: %s", log.String())
	}
}

func TestCancelPreviewStopsWorkersWithoutCachingOrDownloading(t *testing.T) {
	b := &previewBackend{started: make(chan struct{}, 8), stopped: make(chan struct{}, 8), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var log bytes.Buffer
	inv := &core.Invocation{Context: ctx, IO: core.IO{Err: &log}}
	cache := make(map[mediaget.Selection]transferEstimate)
	done := make(chan error, 1)
	go func() {
		done <- estimateOptions(inv, mediaget.Service{Backend: b}, mediaget.Source{URL: "https://example.invalid"},
			[]mediaget.Selection{{Kind: mediaget.Video}, {Kind: mediaget.Audio}}, cache)
	}()
	for range 2 {
		select {
		case <-b.started:
		case <-time.After(3 * time.Second):
			t.Fatal("queries did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not return")
	}
	for range 2 {
		select {
		case <-b.stopped:
		case <-time.After(3 * time.Second):
			t.Fatal("worker survived cancellation")
		}
	}
	if len(cache) != 0 || b.downloads != 0 {
		t.Fatal("cancellation cached results or started a download")
	}
}

type loadingOutput struct {
	mu sync.Mutex
	bytes.Buffer
	spinner chan struct{}
	once    sync.Once
}

func (w *loadingOutput) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if bytes.Contains(p, []byte("\r|")) {
		w.once.Do(func() { close(w.spinner) })
	}
	return w.Buffer.Write(p)
}

func TestLoadingAnimatesWhileWorkIsPending(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w := &loadingOutput{spinner: make(chan struct{})}
	inv := &core.Invocation{Context: ctx, IO: core.IO{Err: w}, Terminal: core.Terminal{StderrTTY: true}}
	value, err := withLoading(inv, "Consultando mídia", func() (int, error) {
		select {
		case <-w.spinner:
			return 42, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	})
	if err != nil || value != 42 {
		t.Fatalf("loading did not animate: %d %v", value, err)
	}
}
