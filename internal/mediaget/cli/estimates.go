package mediacli

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

const (
	estimateConcurrency = 3
	estimateTimeout     = 20 * time.Second
)

type transferEstimate struct {
	bytes int64
	known bool
}

func (e transferEstimate) label() string {
	if !e.known {
		return "tamanho indisponível"
	}
	return "≈ " + mediaget.HumanSize(e.bytes)
}

// Only the calling goroutine renders status. Background queries never write
// to terminal streams or mutate the wizard's cache.
func withLoading[T any](inv *core.Invocation, message string, work func() (T, error)) (T, error) {
	return withLoadingProgress(inv, message, 0, func(func()) (T, error) { return work() })
}

func withLoadingProgress[T any](inv *core.Invocation, message string, total int, work func(func()) (T, error)) (T, error) {
	type result struct {
		value T
		err   error
	}
	done := make(chan result, 1)
	completed := 0
	updates := make(chan struct{}, total)
	status := func() string {
		if total > 0 {
			return fmt.Sprintf("%s: %d/%d", message, completed, total)
		}
		return message + "..."
	}
	fmt.Fprintln(inv.IO.Err, status())
	go func() {
		value, err := work(func() { updates <- struct{}{} })
		done <- result{value, err}
	}()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	frame := 0
	if inv.Terminal.StderrTTY {
		defer fmt.Fprint(inv.IO.Err, "\r\x1b[2K")
	}
	for {
		select {
		case <-inv.Context.Done():
			var zero T
			return zero, inv.Context.Err()
		case r := <-done:
			if err := inv.Context.Err(); err != nil {
				var zero T
				return zero, err
			}
			if r.err == nil && total > 0 {
				completed = total
				if inv.Terminal.StderrTTY {
					fmt.Fprint(inv.IO.Err, "\r\x1b[2K")
				}
				fmt.Fprintln(inv.IO.Err, status())
			}
			return r.value, r.err
		case <-updates:
			completed++
			if !inv.Terminal.StderrTTY && completed < total {
				fmt.Fprintln(inv.IO.Err, status())
			}
		case <-ticker.C:
			if inv.Terminal.StderrTTY {
				fmt.Fprintf(inv.IO.Err, "\r%c %s", "|/-\\"[frame%4], status())
				frame++
			}
		}
	}
}

// Cache includes unavailable estimates so going back does not repeat network
// queries. A metadata timeout is an unavailable estimate; user cancellation
// aborts the wizard. At most three backend inspections run simultaneously.
func estimateOptions(inv *core.Invocation, service mediaget.Service, src mediaget.Source, selections []mediaget.Selection, cache map[mediaget.Selection]transferEstimate) error {
	var pending []mediaget.Selection
	seen := make(map[mediaget.Selection]bool)
	for _, selection := range selections {
		if selection.Kind == mediaget.Subtitle {
			cache[selection] = transferEstimate{}
			continue
		}
		if _, ok := cache[selection]; !ok && !seen[selection] {
			pending = append(pending, selection)
			seen[selection] = true
		}
	}
	if len(pending) == 0 {
		return inv.Context.Err()
	}
	type estimateResult struct {
		estimate transferEstimate
		err      error
	}
	results, err := withLoadingProgress(inv, "Calculando tamanhos estimados", len(pending), func(report func()) ([]estimateResult, error) {
		ctx, cancel := context.WithTimeout(inv.Context, estimateTimeout)
		defer cancel()
		results := make([]estimateResult, len(pending))
		jobs := make(chan int, len(pending))
		for i := range pending {
			jobs <- i
		}
		close(jobs)
		var workers sync.WaitGroup
		for range min(estimateConcurrency, len(pending)) {
			workers.Go(func() {
				for i := range jobs {
					if ctx.Err() != nil {
						return
					}
					info, err := service.Inspect(ctx, src, pending[i])
					results[i].err = err
					if err == nil {
						results[i].estimate.bytes, results[i].estimate.known = mediaget.EstimatedSize(info)
					}
					report()
				}
			})
		}
		workers.Wait()
		return results, inv.Context.Err()
	})
	if err != nil {
		return err
	}
	for i, r := range results {
		if errors.Is(r.err, context.Canceled) {
			return r.err
		}
		cache[pending[i]] = r.estimate
	}
	return nil
}

func printTransferEstimate(inv *core.Invocation, estimate transferEstimate) {
	if estimate.known {
		fmt.Fprintln(inv.IO.Err, "Transferência estimada:", mediaget.HumanSize(estimate.bytes), "(o arquivo final pode variar)")
	} else {
		fmt.Fprintln(inv.IO.Err, "Não foi possível calcular o tamanho; o download pode continuar.")
	}
}
