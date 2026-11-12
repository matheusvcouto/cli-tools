package notionexport

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// MaxConcurrency bounds parallel network operations.
const (
	MinConcurrency     = 1
	MaxConcurrency     = 64
	DefaultConcurrency = 6
)

// ProbeSizes estimates asset sizes without downloading them. file.notion.so
// rejects HEAD, so it requests a single byte and reads the Content-Range total
// (or Content-Length when the server ignores Range). Failures leave the size
// unknown; they never fail the export. The input slice is not modified.
func ProbeSizes(ctx context.Context, client *http.Client, assets []Asset, concurrency int, progress func(done, total int)) []Asset {
	out := append([]Asset(nil), assets...)
	var mu sync.Mutex
	done := 0
	forEachConcurrent(ctx, len(out), concurrency, func(i int) {
		size, contentType := probeOne(ctx, client, out[i].URL)
		mu.Lock()
		out[i].EstimatedSize, out[i].ContentType = size, contentType
		done++
		if progress != nil {
			progress(done, len(out))
		}
		mu.Unlock()
	})
	return out
}

func probeOne(ctx context.Context, client *http.Client, rawURL string) (*int64, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, ""
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, ""
	}
	// Closing without reading aborts the transfer instead of draining a full
	// body when the server ignored Range.
	defer resp.Body.Close()
	contentType := MediaType(resp.Header.Get("Content-Type"))
	switch resp.StatusCode {
	case http.StatusPartialContent:
		return parseContentRangeTotal(resp.Header.Get("Content-Range")), contentType
	case http.StatusOK:
		if resp.ContentLength >= 0 {
			n := resp.ContentLength
			return &n, contentType
		}
		return nil, contentType
	}
	return nil, ""
}

// parseContentRangeTotal parses "bytes 0-0/1749732"; "*" means unknown.
func parseContentRangeTotal(v string) *int64 {
	unit, rest, ok := strings.Cut(strings.TrimSpace(v), " ")
	if !ok || !strings.EqualFold(unit, "bytes") {
		return nil
	}
	_, total, ok := strings.Cut(rest, "/")
	if !ok || total == "*" {
		return nil
	}
	n, err := strconv.ParseInt(total, 10, 64)
	if err != nil || n < 0 {
		return nil
	}
	return &n
}

// forEachConcurrent runs fn for 0..n-1 with at most limit workers. Work not
// yet started is skipped after ctx is done; started work always finishes.
func forEachConcurrent(ctx context.Context, n, limit int, fn func(int)) {
	limit = max(MinConcurrency, min(limit, MaxConcurrency, max(n, 1)))
	next := make(chan int)
	var wg sync.WaitGroup
	for range limit {
		wg.Go(func() {
			for i := range next {
				fn(i)
			}
		})
	}
	for i := range n {
		if ctx.Err() != nil {
			break
		}
		select {
		case next <- i:
		case <-ctx.Done():
		}
	}
	close(next)
	wg.Wait()
}

// GroupSummary aggregates one asset type for prompts and the console summary.
type GroupSummary struct {
	Count        int
	KnownBytes   int64
	UnknownCount int
}

// Summarize groups assets by type.
func Summarize(assets []Asset) map[AssetType]GroupSummary {
	out := map[AssetType]GroupSummary{}
	for _, a := range assets {
		g := out[a.Type]
		g.Count++
		if a.EstimatedSize == nil {
			g.UnknownCount++
		} else {
			g.KnownBytes += *a.EstimatedSize
		}
		out[a.Type] = g
	}
	return out
}
