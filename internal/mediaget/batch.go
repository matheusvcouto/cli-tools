package mediaget

import (
	"context"
	"errors"
	"sync"
)

type BatchEvent struct {
	Index             int
	Progress          Progress
	Started, Finished bool
	Result            Result
	Err               error
}
type BatchResult struct {
	Started bool
	Result  Result
	Err     error
}

// DownloadBatch owns scheduling only. Each request uses the same confined
// publication path as a single download. notify is called on the caller's
// goroutine, never concurrently; cancellation joins every worker.
func (s Service) DownloadBatch(ctx context.Context, requests []Request, jobs int, stopOnError bool, notify func(BatchEvent) error) ([]BatchResult, error) {
	if jobs < 1 || jobs > 8 {
		return nil, errors.New("jobs deve estar entre 1 e 8")
	}
	// Recheck all immutable requests before dispatch; callers other than the
	// CLI must not accidentally begin a partially invalid batch.
	for _, req := range requests {
		if err := ValidateSource(req.Source); err != nil {
			return nil, err
		}
		_, sel := OutputSelection(req.Name, req.Selection)
		if err := ValidateSelection(sel); err != nil {
			return nil, err
		}
		if err := s.ValidateDestination(req.OutputDir); err != nil {
			return nil, err
		}
		if err := s.Check(ctx, sel); err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make([]BatchResult, len(requests))
	events := make(chan BatchEvent, jobs*2)
	var mu sync.Mutex
	next, stopped := 0, false
	var wg sync.WaitGroup
	for range min(jobs, len(requests)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				mu.Lock()
				if stopped || next == len(requests) || ctx.Err() != nil {
					mu.Unlock()
					return
				}
				i := next
				next++
				mu.Unlock()
				events <- BatchEvent{Index: i, Started: true}
				result, err := s.Download(ctx, requests[i], func(p Progress) {
					select {
					case events <- BatchEvent{Index: i, Progress: p}:
					case <-ctx.Done():
					}
				})
				if err != nil && stopOnError {
					mu.Lock()
					stopped = true
					mu.Unlock()
				}
				events <- BatchEvent{Index: i, Finished: true, Result: result, Err: err}
			}
		}()
	}
	go func() { wg.Wait(); close(events) }()
	var outputErr error
	for e := range events {
		if e.Started {
			results[e.Index].Started = true
		}
		if e.Finished {
			results[e.Index].Result = e.Result
			results[e.Index].Err = e.Err
		}
		if notify != nil && outputErr == nil {
			if err := notify(e); err != nil {
				outputErr = err
				cancel()
			}
		}
	}
	if outputErr != nil {
		return results, outputErr
	}
	if err := ctx.Err(); err != nil {
		return results, err
	}
	for _, result := range results {
		if result.Err != nil {
			return results, errors.New("um ou mais downloads do lote falharam")
		}
	}
	return results, nil
}
