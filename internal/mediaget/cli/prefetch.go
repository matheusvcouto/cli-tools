package mediacli

import (
	"context"
	"fmt"
	"sync"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type estimateEntry struct {
	estimate transferEstimate
	ready    bool
}
type estimateSession struct {
	mu      sync.Mutex
	entries map[mediaget.Selection]estimateEntry
	ctx     context.Context
	cancel  context.CancelFunc
	workers sync.WaitGroup
	jobs    chan mediaget.Selection
}

// One bounded, memory-only session per source. Queries never draw on the terminal.
func newEarlyEstimateSession(ctx context.Context, service mediaget.Service, src mediaget.Source) *estimateSession {
	ctx, cancel := context.WithTimeout(ctx, estimateTimeout)
	session := &estimateSession{entries: make(map[mediaget.Selection]estimateEntry), ctx: ctx, cancel: cancel, jobs: make(chan mediaget.Selection, 16)}
	for range estimateConcurrency {
		session.workers.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case sel := <-session.jobs:
					if ctx.Err() != nil {
						return
					}
					var estimate transferEstimate
					metadata, err := service.Inspect(ctx, src, sel)
					if err == nil {
						estimate = estimateFromInfo(metadata)
					}
					session.mu.Lock()
					session.entries[sel] = estimateEntry{estimate: estimate, ready: true}
					session.mu.Unlock()
				}
			}
		})
	}
	session.enqueue(mediaget.Selection{Kind: mediaget.Video}, mediaget.Selection{Kind: mediaget.Audio})
	return session
}
func (s *estimateSession) enqueue(selections ...mediaget.Selection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sel := range selections {
		if _, exists := s.entries[sel]; exists || sel.Kind == mediaget.Subtitle {
			continue
		}
		s.entries[sel] = estimateEntry{}
		select {
		case s.jobs <- sel:
		case <-s.ctx.Done():
		}
	}
}
func (s *estimateSession) close() { s.cancel(); s.workers.Wait() }
func (s *estimateSession) get(sel mediaget.Selection) estimateEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sel.Kind == mediaget.Subtitle {
		return estimateEntry{ready: true}
	}
	entry := s.entries[sel]
	if !entry.ready && s.ctx.Err() != nil {
		entry.ready = true
	}
	return entry
}
func (s *estimateSession) labels(selections []mediaget.Selection, base []string) []string {
	labels := append([]string(nil), base...)
	for i, sel := range selections {
		entry := s.get(sel)
		label := fmt.Sprintf("%c calculando…", "|/-\\"[int(time.Now().UnixMilli()/200)%4])
		if entry.ready {
			label = entry.estimate.label()
		}
		labels[i] += " — " + label
	}
	return labels
}

func chooseLive(inv *core.Invocation, message string, snapshot func() []string, back bool) (int, error) {
	if native, ok := inv.Interaction.(interface {
		SelectLive(context.Context, string, []string, bool, func() []string) (int, error)
	}); ok {
		index, err := native.SelectLive(inv.Context, message, snapshot(), back, snapshot)
		if err != errTextSelect {
			return index, err
		}
	}
	// Redirected/dumb terminals get a snapshot without concurrent output while
	// their text reader waits. Remaining estimates still run in the background.
	return choose(inv, message, snapshot(), back)
}

func videoQualities(info mediaget.Info) ([]int, []string) {
	heights := []int{0}
	labels := []string{"Melhor qualidade disponível"}
	maxHeight := 0
	for _, h := range info.Heights {
		maxHeight = max(maxHeight, h)
	}
	if !info.UnknownVideoHeight && maxHeight > 0 {
		labels[0] += fmt.Sprintf(" (fonte até %dp)", maxHeight)
	}
	for _, height := range []int{2160, 1080, 720, 480, 360} {
		if !info.UnknownVideoHeight && maxHeight > 0 && height >= maxHeight {
			continue
		}
		available := len(info.Heights) == 0
		for _, h := range info.Heights {
			if h <= height {
				available = true
				break
			}
		}
		if available {
			heights = append(heights, height)
			labels = append(labels, fmt.Sprintf("Até %dp", height))
		}
	}
	return heights, labels
}

func scheduleQualities(inv *core.Invocation, session *estimateSession, info mediaget.Info) {
	heights, _ := videoQualities(info)
	var selections []mediaget.Selection
	for _, height := range heights {
		selections = append(selections, mediaget.Selection{Kind: mediaget.Video, Height: height})
	}
	if inv.Present(prefix + "quality") {
		height, _ := core.ValueAs[int](inv, prefix+"quality")
		selections = append(selections, mediaget.Selection{Kind: mediaget.Video, Height: height})
	}
	session.enqueue(selections...)
}
