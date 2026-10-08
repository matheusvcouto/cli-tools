package mediaget

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type batchBackend struct {
	mu                  sync.Mutex
	active, peak, calls int
	block               bool
	started             chan struct{}
	release             chan struct{}
	fail                bool
}

func (b *batchBackend) Check(context.Context, Selection) error                   { return nil }
func (b *batchBackend) Inspect(context.Context, Source, Selection) (Info, error) { return Info{}, nil }
func (b *batchBackend) Download(ctx context.Context, r Request, work string, progress func(Progress)) error {
	b.mu.Lock()
	b.calls++
	b.active++
	b.peak = max(b.peak, b.active)
	b.mu.Unlock()
	defer func() { b.mu.Lock(); b.active--; b.mu.Unlock() }()
	if b.started != nil {
		b.started <- struct{}{}
	}
	if b.block {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-b.release:
		}
	}
	if b.fail {
		return errors.New("synthetic failure")
	}
	if progress != nil {
		progress(Progress{Downloaded: 3})
	}
	return os.WriteFile(filepath.Join(work, "media.mp4"), []byte("synthetic"), 0600)
}
func batchRequests(t *testing.T, n int) []Request {
	t.Helper()
	dir := t.TempDir()
	rs := make([]Request, n)
	for i := range rs {
		rs[i] = Request{Source: Source{URL: "https://example.invalid"}, Selection: Selection{Kind: Video}, OutputDir: dir, Name: "duplicate"}
	}
	return rs
}
func TestBatchBoundsWorkersAndPublishesWithoutClobber(t *testing.T) {
	b := &batchBackend{block: true, started: make(chan struct{}, 4), release: make(chan struct{})}
	s := Service{Backend: b}
	requests := batchRequests(t, 4)
	type done struct {
		results []BatchResult
		err     error
	}
	ch := make(chan done, 1)
	go func() { r, e := s.DownloadBatch(t.Context(), requests, 2, false, nil); ch <- done{r, e} }()
	<-b.started
	<-b.started
	b.mu.Lock()
	calls := b.calls
	b.mu.Unlock()
	if calls != 2 {
		t.Fatal(calls)
	}
	close(b.release)
	result := <-ch
	if result.err != nil {
		t.Fatal(result.err)
	}
	paths := map[string]bool{}
	for _, r := range result.results {
		if !r.Started || r.Err != nil || paths[r.Result.Path] {
			t.Fatalf("%+v", r)
		}
		paths[r.Result.Path] = true
	}
	if b.peak != 2 || b.active != 0 {
		t.Fatalf("peak=%d active=%d", b.peak, b.active)
	}
}
func TestBatchCancelJoinsAllActiveAndStopsQueue(t *testing.T) {
	b := &batchBackend{block: true, started: make(chan struct{}, 4)}
	s := Service{Backend: b}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	started := 0
	results, err := s.DownloadBatch(ctx, batchRequests(t, 4), 2, false, func(e BatchEvent) error {
		if e.Started {
			started++
			if started == 2 {
				cancel()
			}
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) || b.active != 0 {
		t.Fatal(err, b.active)
	}
	for _, r := range results {
		if r.Started && (!errors.Is(r.Err, context.Canceled) || r.Result.Path != "") {
			t.Fatalf("%+v", r)
		}
	}
}
func TestBatchStopAndRendererFailure(t *testing.T) {
	b := &batchBackend{fail: true}
	s := Service{Backend: b}
	r, err := s.DownloadBatch(t.Context(), batchRequests(t, 4), 1, true, nil)
	if err == nil || b.calls != 1 || r[1].Started {
		t.Fatal(err, b.calls, r)
	}
	b = &batchBackend{block: true, started: make(chan struct{}, 4)}
	s.Backend = b
	want := errors.New("synthetic writer failure")
	_, err = s.DownloadBatch(t.Context(), batchRequests(t, 4), 2, false, func(BatchEvent) error { return want })
	if !errors.Is(err, want) || b.active != 0 {
		t.Fatal(err, b.active)
	}
}

func TestBatchRejectsInvalidRequestBeforeAnyMutation(t *testing.T) {
	b := &batchBackend{}
	s := Service{Backend: b}
	reqs := batchRequests(t, 2)
	reqs[1].Source.Origin = "https://example.invalid/path"
	if _, err := s.DownloadBatch(t.Context(), reqs, 2, false, nil); err == nil || b.calls != 0 {
		t.Fatal(err, b.calls)
	}
}
