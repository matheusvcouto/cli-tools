package mediacli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

func TestBatchPanelRecyclesSlotsAndKeepsProcessingActive(t *testing.T) {
	var log bytes.Buffer
	inv := &core.Invocation{Context: t.Context(), IO: core.IO{Err: &log, Out: io.Discard}, Terminal: core.Terminal{StderrTTY: true, Width: 64}}
	entries := []batchEntry{{req: mediaget.Request{Name: "first"}}, {req: mediaget.Request{Name: "second"}}, {req: mediaget.Request{Name: "third"}}}
	r := newBatchProgressRenderer(inv, []int{0, 1, 2}, entries, 2)
	r.accept(mediaget.BatchEvent{Index: 0, Started: true})
	r.accept(mediaget.BatchEvent{Index: 1, Started: true})
	r.accept(mediaget.BatchEvent{Index: 0, Progress: mediaget.Progress{Stage: mediaget.Processing}})
	r.draw(true)
	first := log.String()
	log.Reset()
	r.draw(true)
	if first == log.String() || !strings.Contains(first, "Processando mídia") || !strings.Contains(first, "0/3 finalizados") {
		t.Fatal(first, log.String())
	}
	r.accept(mediaget.BatchEvent{Index: 0, Finished: true})
	r.accept(mediaget.BatchEvent{Index: 2, Started: true})
	if r.slots[0] != 2 || r.slots[1] != 1 || len(r.active) != 2 || r.completed != 1 {
		t.Fatal(r.slots, r.active, r.completed)
	}
	log.Reset()
	r.draw(true)
	if !strings.Contains(log.String(), "third") || strings.Contains(log.String(), "first") {
		t.Fatal(log.String())
	}
	for _, line := range strings.Split(log.String(), "\n") {
		if !strings.Contains(line, "\x1b") && displayCells(line) > 63 {
			t.Fatal(line)
		}
	}
	r.accept(mediaget.BatchEvent{Index: 1, Finished: true})
	r.accept(mediaget.BatchEvent{Index: 2, Finished: true})
	r.settled = true
	log.Reset()
	r.draw(true)
	if strings.Contains(log.String(), "Aguardando") || !strings.Contains(log.String(), "100%") {
		t.Fatal(log.String())
	}
}

func TestBatchRedirectedProgressHasNoTerminalEscapes(t *testing.T) {
	var log bytes.Buffer
	inv := &core.Invocation{IO: core.IO{Err: &log, Out: io.Discard}}
	r := newBatchProgressRenderer(inv, []int{0}, []batchEntry{{req: mediaget.Request{Name: "synthetic"}}}, 1)
	r.accept(mediaget.BatchEvent{Started: true})
	r.draw(true)
	r.accept(mediaget.BatchEvent{Progress: mediaget.Progress{Stage: mediaget.Transferring, Downloaded: 10}})
	r.draw(true)
	r.accept(mediaget.BatchEvent{Finished: true, Err: errors.New("synthetic")})
	r.draw(true)
	if strings.Contains(log.String(), "\x1b") || !strings.Contains(log.String(), "1 falhas") || !strings.Contains(log.String(), "total desconhecido") {
		t.Fatal(log.String())
	}
}

type progressFailureWriter struct {
	writes  int
	started <-chan struct{}
}

func (w *progressFailureWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		if w.started != nil {
			<-w.started
		}
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

type waitingProgressBackend struct {
	joined  chan struct{}
	started chan struct{}
}

func (b *waitingProgressBackend) Check(context.Context, mediaget.Selection) error { return nil }
func (b *waitingProgressBackend) Inspect(context.Context, mediaget.Source, mediaget.Selection) (mediaget.Info, error) {
	return mediaget.Info{}, nil
}
func (b *waitingProgressBackend) Download(ctx context.Context, _ mediaget.Request, _ string, p func(mediaget.Progress)) error {
	defer close(b.joined)
	close(b.started)
	p(mediaget.Progress{Stage: mediaget.Processing})
	<-ctx.Done()
	return ctx.Err()
}
func TestBatchOutputFailureCancelsAndJoinsActiveTransfer(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	b := &waitingProgressBackend{joined: make(chan struct{}), started: make(chan struct{})}
	req := mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid"}, Selection: mediaget.Selection{Kind: mediaget.Video}, Name: "synthetic", OutputDir: t.TempDir()}
	// Plain initial draw and start log succeed; the next processing tick fails.
	w := &progressFailureWriter{writes: -1, started: b.started}
	inv := &core.Invocation{Context: ctx, IO: core.IO{Err: w, Out: io.Discard}, Terminal: core.Terminal{StderrTTY: true, Width: 80}}
	_, err := downloadBatchWithProgress(inv, mediaget.Service{Backend: b}, []mediaget.Request{req}, []int{0}, []batchEntry{{req: req}}, 1, false)
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	select {
	case <-b.joined:
	default:
		t.Fatal("transfer was not joined")
	}
}
