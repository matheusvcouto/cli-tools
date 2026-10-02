package mediacli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
	"golang.org/x/term"
)

type downloadRenderer struct {
	inv               *core.Invocation
	current           mediaget.Progress
	frame             int
	last              time.Time
	stage             mediaget.ProgressStage
	err               error
	forecast          transferEstimate
	streams, expected map[string]mediaget.Progress
}

func newDownloadRenderer(inv *core.Invocation) *downloadRenderer { return &downloadRenderer{inv: inv} }
func (r *downloadRenderer) render(force bool) {
	if r.err != nil {
		return
	}
	now := time.Now()
	if !force && !r.inv.Terminal.StderrTTY && now.Sub(r.last) < 5*time.Second {
		return
	}
	r.last = now
	p := r.current
	message := "Baixando mídia..."
	switch p.Stage {
	case mediaget.Processing:
		message = "Processando mídia (conversão/mesclagem)..."
	case mediaget.Publishing:
		message = "Verificando e salvando arquivo..."
	}
	width := r.inv.Terminal.Width
	if f, ok := r.inv.IO.Err.(*os.File); ok && r.inv.Terminal.StderrTTY {
		width, _, _ = term.GetSize(int(f.Fd()))
	}
	if width <= 0 {
		width = 80
	}
	if p.Stage == mediaget.Transferring && (p.Downloaded > 0 || p.Total > 0) {
		sizes := mediaget.HumanSize(p.Downloaded)
		percent := -1
		if p.Total > 0 {
			percent = int(min(100.0, float64(p.Downloaded)*100/float64(p.Total)))
			if p.Estimated {
				percent = min(99, percent)
			}
			total := mediaget.HumanSize(p.Total)
			if p.Estimated {
				total = "≈ " + total
			}
			sizes += " / " + total
		} else {
			sizes += " / total desconhecido"
		}
		stats := sizes
		if p.Speed > 0 {
			stats += " | " + mediaget.HumanSize(int64(p.Speed)) + "/s"
		}
		if p.ETA > 0 {
			stats += " | " + (time.Duration(p.ETA) * time.Second).String()
		}
		if r.inv.Terminal.StderrTTY {
			if percent >= 0 {
				barWidth := min(24, max(5, width-len(stats)-10))
				filled := barWidth * percent / 100
				message = fmt.Sprintf("[%s%s] %3d%% %s", strings.Repeat("█", filled), strings.Repeat("░", barWidth-filled), percent, stats)
				if p.Downloaded == 0 {
					message = fmt.Sprintf("%c %s", "|/-\\"[r.frame%4], message)
				}
			} else {
				width := min(18, max(5, width-len(stats)-4))
				position := r.frame % width
				message = fmt.Sprintf("[%s█%s] %s", strings.Repeat("░", position), strings.Repeat("░", width-position-1), stats)
			}
		} else {
			message = "Transferência atual: " + mediaget.HumanSize(p.Downloaded)
			if percent >= 0 {
				message = fmt.Sprintf("Transferência atual: %d%% (%s)", percent, sizes)
			}
			if p.Speed > 0 {
				message += " | " + mediaget.HumanSize(int64(p.Speed)) + "/s"
			}
			if p.ETA > 0 {
				message += " | restante: " + (time.Duration(p.ETA) * time.Second).String()
			}
		}
	} else if r.inv.Terminal.StderrTTY {
		message = fmt.Sprintf("%c %s", "|/-\\"[r.frame%4], message)
	}
	r.frame++
	if r.inv.Terminal.StderrTTY {
		_, r.err = fmt.Fprintf(r.inv.IO.Err, "\r\x1b[2K%s", fitLine(message, width-1))
	} else {
		_, r.err = fmt.Fprintln(r.inv.IO.Err, message)
	}
}

// Only this goroutine draws. yt-dlp callbacks feed bounded events, and the ticker
// keeps processing/unknown-total stages visibly alive without inventing progress.
func downloadWithProgress(inv *core.Invocation, service mediaget.Service, req mediaget.Request, r *downloadRenderer) (mediaget.Result, error) {
	type outcome struct {
		result mediaget.Result
		err    error
	}
	ctx, cancel := context.WithCancel(inv.Context)
	defer cancel()
	done := make(chan outcome, 1)
	events := make(chan mediaget.Progress, 32)
	go func() {
		result, err := service.Download(ctx, req, func(p mediaget.Progress) {
			select {
			case events <- p:
			case <-ctx.Done():
			}
		})
		done <- outcome{result, err}
	}()
	r.accept(mediaget.Progress{Stage: mediaget.Transferring})
	r.stage = mediaget.Transferring
	r.render(true)
	ticker := time.NewTicker(125 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		if inv.Terminal.StderrTTY {
			if r.stage != "" {
				fmt.Fprintln(inv.IO.Err)
			}
		}
	}()
	for {
		if r.err != nil {
			cancel()
			final := <-done
			return final.result, errors.Join(r.err, final.err)
		}
		select {
		case p := <-events:
			changed := p.Stage != r.stage || r.current.Downloaded == 0 && p.Downloaded > 0
			r.accept(p)
			r.stage = p.Stage
			if changed {
				r.render(true)
			}
		case <-ticker.C:
			if r.stage != "" {
				r.render(false)
			}
		case final := <-done:
			// The backend can finish before buffered stage events are consumed.
			for len(events) > 0 {
				p := <-events
				changed := p.Stage != r.stage || r.current.Downloaded == 0 && p.Downloaded > 0
				r.accept(p)
				r.stage = p.Stage
				r.render(changed)
			}
			return final.result, errors.Join(final.err, r.err)
		}
	}
}
