package notioncli

import (
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	ne "github.com/matheusvcouto/cli-tools/internal/notionexport"
)

// downloadReporter renders download progress. On a terminal it keeps one
// live line (count, bytes, estimated total) and prints only failures above
// it; otherwise it writes one line per event so logs stay readable.
type downloadReporter struct {
	out      io.Writer
	live     bool
	total    int64 // sum of known estimates, 0 when unknown
	received atomic.Int64

	mu        sync.Mutex
	completed int
	count     int
	drawn     bool
	stop      chan struct{}
	done      chan struct{}
}

func newDownloadReporter(inv *core.Invocation, selected []ne.Asset) *downloadReporter {
	r := &downloadReporter{out: inv.IO.Err, live: inv.Terminal.StderrTTY, count: len(selected)}
	for _, a := range selected {
		if a.EstimatedSize == nil {
			r.total = 0
			break
		}
		r.total += *a.EstimatedSize
	}
	if r.live && r.count > 0 {
		r.stop, r.done = make(chan struct{}), make(chan struct{})
		go r.loop()
	}
	return r
}

func (r *downloadReporter) loop() {
	defer close(r.done)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.mu.Lock()
			r.draw()
			r.mu.Unlock()
		}
	}
}

// draw must be called with mu held.
func (r *downloadReporter) draw() {
	size := FormatBytes(max(0, r.received.Load()))
	if r.total > 0 {
		size += " de " + FormatBytes(r.total)
	}
	fmt.Fprintf(r.out, "\r\x1b[2KBaixando assets: %d/%d · %s", r.completed, r.count, size)
	r.drawn = true
}

func (r *downloadReporter) clear() {
	if r.drawn {
		fmt.Fprint(r.out, "\r\x1b[2K")
		r.drawn = false
	}
}

func (r *downloadReporter) bytes(n int64) { r.received.Add(n) }

func (r *downloadReporter) event(ev ne.DownloadEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := sanitize(ne.OriginalFilename(firstNonEmpty(ev.Asset.Source, ev.Asset.URL)))
	if ev.Started {
		if !r.live {
			fmt.Fprintf(r.out, "Baixando %s: %s\n", ev.Asset.Type, name)
		}
		return
	}
	r.completed = ev.Completed
	if ev.Result.Status == ne.StatusFailed && ev.Result.Err != nil {
		r.clear()
		fmt.Fprintf(r.out, "Falha em %s: %s\n", name, sanitize(ev.Result.Err.Error()))
	}
	if r.live {
		r.draw()
	} else {
		fmt.Fprintf(r.out, "Downloads: %d/%d\n", ev.Completed, ev.Total)
	}
}

// finish stops the live line and leaves the terminal on a clean line.
func (r *downloadReporter) finish() {
	if r.stop != nil {
		close(r.stop)
		<-r.done
		r.stop = nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.live && r.drawn {
		r.draw()
		fmt.Fprintln(r.out)
		r.drawn = false
	}
}
