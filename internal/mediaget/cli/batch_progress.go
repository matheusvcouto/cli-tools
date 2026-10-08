package mediacli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type batchProgressRenderer struct {
	inv                             *core.Invocation
	indices                         []int
	entries                         []batchEntry
	slots                           []int
	active                          map[int]*downloadRenderer
	total, completed, failed, lines int
	settled                         bool
	last                            time.Time
	err                             error
}

func newBatchProgressRenderer(inv *core.Invocation, indices []int, entries []batchEntry, jobs int) *batchProgressRenderer {
	slots := make([]int, min(jobs, len(indices)))
	for i := range slots {
		slots[i] = -1
	}
	return &batchProgressRenderer{inv: inv, indices: indices, entries: entries, slots: slots, active: map[int]*downloadRenderer{}, total: len(indices)}
}
func (r *batchProgressRenderer) clear() {
	if r.lines == 0 || r.err != nil {
		return
	}
	_, r.err = fmt.Fprintf(r.inv.IO.Err, "\x1b[%dA\r\x1b[J", r.lines)
	r.lines = 0
}
func (r *batchProgressRenderer) accept(e mediaget.BatchEvent) {
	if r.err != nil {
		return
	}
	entry := r.entries[r.indices[e.Index]]
	if e.Started {
		d := newDownloadRenderer(r.inv)
		d.setEstimate(entry.estimate)
		d.accept(mediaget.Progress{Stage: mediaget.Transferring})
		r.active[e.Index] = d
		for i, v := range r.slots {
			if v < 0 {
				r.slots[i] = e.Index
				break
			}
		}
		if !r.inv.Terminal.StderrTTY {
			_, r.err = fmt.Fprintf(r.inv.IO.Err, "Iniciando item %d: %s\n", r.indices[e.Index]+1, mediaget.SafeName(entry.req.Name))
		}
	} else if e.Finished {
		r.clear()
		delete(r.active, e.Index)
		for i, v := range r.slots {
			if v == e.Index {
				r.slots[i] = -1
			}
		}
		r.completed++
		state := "concluído"
		if e.Err != nil {
			r.failed++
			state = "falhou"
		}
		if r.err == nil {
			_, r.err = fmt.Fprintf(r.inv.IO.Err, "Item %d %s: %s\n", r.indices[e.Index]+1, state, mediaget.SafeName(entry.req.Name))
		}
		if r.err == nil && e.Err != nil {
			width := r.inv.Terminal.Width
			if width <= 0 {
				width = 80
			}
			_, r.err = fmt.Fprintln(r.inv.IO.Err, "  "+fitLine(e.Err.Error(), width-3))
		}
		if r.err == nil && e.Err == nil {
			_, r.err = fmt.Fprintln(r.inv.IO.Out, e.Result.Path)
			if r.err == nil && e.Result.CleanupWarning != nil {
				_, r.err = fmt.Fprintln(r.inv.IO.Err, "Download concluído; não foi possível limpar a área de trabalho:", e.Result.CleanupWarning)
			}
		}
	} else if d := r.active[e.Index]; d != nil {
		d.accept(e.Progress)
	}
}
func (r *batchProgressRenderer) draw(force bool) {
	if r.err != nil {
		return
	}
	tty := r.inv.Terminal.StderrTTY
	if !tty && !force && time.Since(r.last) < 5*time.Second {
		return
	}
	r.last = time.Now()
	width := r.inv.Terminal.Width
	if width <= 0 {
		width = 80
	}
	percent := 0
	if r.total > 0 {
		percent = r.completed * 100 / r.total
	}
	summary := fmt.Sprintf("Lote: %d/%d finalizados · %d ativos · %d falhas", r.completed, r.total, len(r.active), r.failed)
	if tty {
		filled := 20 * percent / 100
		summary = fmt.Sprintf("[%s%s] %3d%% do lote", strings.Repeat("█", filled), strings.Repeat("░", 20-filled), percent)
		r.clear()
	}
	lines := []string{summary}
	if tty {
		lines = append(lines, fmt.Sprintf("%d/%d finalizados · %d ativos · %d falhas", r.completed, r.total, len(r.active), r.failed))
	}
	for _, index := range r.slots {
		if index < 0 {
			if tty && !r.settled {
				lines = append(lines, "  Aguardando próximo item...", "")
			}
			continue
		}
		entry := r.entries[r.indices[index]]
		lines = append(lines, fmt.Sprintf("  %d. %s", r.indices[index]+1, mediaget.SafeName(entry.req.Name)), "  "+r.active[index].line(width-2, tty))
	}
	if r.err != nil {
		return
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(fitLine(line, width-1))
		b.WriteByte('\n')
	}
	_, r.err = fmt.Fprint(r.inv.IO.Err, b.String())
	if tty {
		r.lines = len(lines)
	}
}

// One owner renders; bounded events and a ticker keep every active stage visible.
// On output failure, cancel and join all transfers before returning.
func downloadBatchWithProgress(inv *core.Invocation, service mediaget.Service, requests []mediaget.Request, indices []int, entries []batchEntry, jobs int, stop bool) ([]mediaget.BatchResult, error) {
	type outcome struct {
		results []mediaget.BatchResult
		err     error
	}
	ctx, cancel := context.WithCancel(inv.Context)
	defer cancel()
	events := make(chan mediaget.BatchEvent, 32)
	done := make(chan outcome, 1)
	r := newBatchProgressRenderer(inv, indices, entries, jobs)
	r.draw(true)
	if r.err != nil {
		return nil, r.err
	}
	go func() {
		results, err := service.DownloadBatch(ctx, requests, jobs, stop, func(e mediaget.BatchEvent) error {
			select {
			case events <- e:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		done <- outcome{results, err}
	}()
	ticker := time.NewTicker(125 * time.Millisecond)
	defer ticker.Stop()
	for {
		if r.err != nil {
			cancel()
			final := <-done
			return final.results, errors.Join(r.err, final.err)
		}
		select {
		case e := <-events:
			r.accept(e)
			r.draw(e.Started || e.Finished)
		case <-ticker.C:
			r.draw(false)
		case final := <-done:
			for len(events) > 0 {
				r.accept(<-events)
			}
			r.settled = true
			r.draw(true)
			// Leave the last panel in scrollback, with the cursor below it.
			return final.results, errors.Join(final.err, r.err)
		}
	}
}
