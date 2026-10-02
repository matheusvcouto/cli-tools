package mediacli

import (
	"math"

	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

// Forecasts describe the whole selected transfer. Per-format opaque identities
// let us add video/audio bytes once without confusing a stream total with the whole.
func (r *downloadRenderer) setEstimate(e transferEstimate) {
	r.forecast = e
	r.streams = make(map[string]mediaget.Progress)
	r.expected = make(map[string]mediaget.Progress)
	if !e.known {
		return
	}
	for _, part := range e.parts {
		n, ok := mediaget.EstimatedSize(mediaget.Info{Size: part, Duration: e.duration})
		if !ok || part.StreamID == "" || len(r.expected) >= 16 {
			r.expected = nil
			return
		}
		if _, duplicate := r.expected[part.StreamID]; duplicate {
			r.expected = nil
			return
		}
		r.expected[part.StreamID] = mediaget.Progress{Total: n, Estimated: true}
	}
}
func (r *downloadRenderer) accept(p mediaget.Progress) {
	if p.Stage != mediaget.Transferring || !r.forecast.known {
		r.current = p
		return
	}
	if p.StreamID != "" {
		if r.streams == nil {
			r.streams = make(map[string]mediaget.Progress)
		}
		// The supported video/audio selections require at most a few streams. Never
		// allocate indefinitely for identifiers from an untrusted external process.
		if _, exists := r.streams[p.StreamID]; exists || len(r.streams) < 16 {
			r.streams[p.StreamID] = p
		}
		downloaded := int64(0)
		total := int64(0)
		known := len(r.expected) > 0
		estimated := false
		for id, event := range r.streams {
			if downloaded > math.MaxInt64-event.Downloaded {
				known = false
				downloaded = p.Downloaded
				break
			}
			downloaded += event.Downloaded
			if _, ok := r.expected[id]; !ok {
				known = false
			}
		}
		for id, expected := range r.expected {
			if event, ok := r.streams[id]; ok && event.Total > 0 {
				expected = event
			}
			if total > math.MaxInt64-expected.Total {
				known = false
				break
			}
			total += expected.Total
			estimated = estimated || expected.Estimated
		}
		p.Downloaded = downloaded
		if known {
			p.Total = total
			p.Estimated = estimated
		} else {
			p.Total = r.forecast.bytes
			p.Estimated = true
		}
		// yt-dlp's ETA refers to its current stream. Compute a whole-transfer estimate
		// from remaining bytes/speed instead when using whole-transfer totals.
		p.ETA = 0
		if p.Speed > 0 && p.Total > p.Downloaded {
			seconds := float64(p.Total-p.Downloaded) / p.Speed
			if seconds < 365*24*60*60 {
				p.ETA = int(math.Ceil(seconds))
			}
		}
	} else if p.Total <= 0 {
		// No stream identity: show the forecast, without guessing stream boundaries.
		p.Total = r.forecast.bytes
		p.Estimated = true
	}
	r.current = p
}
