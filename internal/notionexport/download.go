package notionexport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

// DefaultStallTimeout aborts a transfer that receives no bytes for this long.
const DefaultStallTimeout = 60 * time.Second

// DownloadStatus is the per-asset outcome recorded in manifest.json.
type DownloadStatus string

const (
	StatusDownloaded DownloadStatus = "downloaded"
	StatusSkipped    DownloadStatus = "skipped"
	StatusFailed     DownloadStatus = "failed"
)

// DownloadResult describes one selected asset after the transfer phase.
type DownloadResult struct {
	Status      DownloadStatus
	Path        string // relative to the export directory, slash-separated
	Bytes       int64
	ContentType string
	Err         error
}

// Downloader transfers assets into an export staging root.
type Downloader struct {
	Client       *http.Client
	Retry        RetryPolicy
	Concurrency  int
	StallTimeout time.Duration
	// Progress receives serialized start/finish notifications.
	Progress func(DownloadEvent)
	// OnBytes receives received byte counts; it may be called concurrently
	// and must be cheap (e.g. an atomic add). A retried transfer reports a
	// negative count for the bytes it discarded.
	OnBytes func(int64)
}

// DownloadEvent reports progress without exposing transport details.
type DownloadEvent struct {
	Asset     Asset
	Started   bool
	Result    DownloadResult
	Completed int
	Total     int
}

// Download fetches the assets into root/assets/<type>/. Each file is created
// exclusively (never replacing anything) and removed if its transfer fails;
// one failure never cancels the others. Cancellation of ctx stops new work and
// interrupts transfers in progress.
func (d Downloader) Download(ctx context.Context, root *safefs.Root, assets []Asset) map[string]DownloadResult {
	results := make(map[string]DownloadResult, len(assets))
	var mu sync.Mutex
	completed := 0
	notify := func(ev DownloadEvent) {
		if d.Progress != nil {
			d.Progress(ev)
		}
	}
	forEachConcurrent(ctx, len(assets), d.Concurrency, func(i int) {
		asset := assets[i]
		mu.Lock()
		notify(DownloadEvent{Asset: asset, Started: true, Completed: completed, Total: len(assets)})
		mu.Unlock()
		result := d.downloadWithRetry(ctx, root, asset)
		mu.Lock()
		results[asset.BlockID] = result
		completed++
		notify(DownloadEvent{Asset: asset, Result: result, Completed: completed, Total: len(assets)})
		mu.Unlock()
	})
	for _, asset := range assets {
		if _, ok := results[asset.BlockID]; !ok {
			err := ctx.Err()
			if err == nil {
				err = errors.New("download não executado")
			}
			results[asset.BlockID] = DownloadResult{Status: StatusFailed, Err: err}
		}
	}
	return results
}

func (d Downloader) downloadWithRetry(ctx context.Context, root *safefs.Root, asset Asset) DownloadResult {
	var result DownloadResult
	err := d.Retry.Do(ctx, func(int) (bool, error) {
		var retry bool
		var err error
		result, retry, err = d.downloadOne(ctx, root, asset)
		return retry, err
	})
	if err != nil {
		return DownloadResult{Status: StatusFailed, Err: err}
	}
	return result
}

func (d Downloader) downloadOne(ctx context.Context, root *safefs.Root, asset Asset) (DownloadResult, bool, error) {
	if !IsHTTPURL(asset.URL) {
		return DownloadResult{}, false, errors.New("URL do asset não é http(s)")
	}
	stall := d.StallTimeout
	if stall <= 0 {
		stall = DefaultStallTimeout
	}
	reqCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return DownloadResult{}, false, fmt.Errorf("URL inválida: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := d.Client.Do(req)
	if err != nil {
		return DownloadResult{}, TransientNetworkError(ctx, err), err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		status := NewStatusError(resp)
		return DownloadResult{}, RetryableStatus(resp.StatusCode), status
	}
	contentType := resp.Header.Get("Content-Type")
	if err := CheckContentType(asset.Type, contentType); err != nil {
		return DownloadResult{}, false, err
	}
	name := SafeAssetName(firstNonEmpty(asset.Source, asset.URL), asset.Index)
	if path.Ext(name) == "" {
		name += ExtensionForContentType(contentType)
	}
	dir := path.Join("assets", AssetDir(asset.Type))
	if err := root.MkdirAll(dir, 0o755); err != nil {
		return DownloadResult{}, false, fmt.Errorf("criar pasta de assets: %w", err)
	}
	rel := path.Join(dir, name)
	f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return DownloadResult{}, false, fmt.Errorf("criar %s: %w", rel, err)
	}
	written, copyErr := copyWithStall(reqCtx, cancel, f, resp.Body, stall, d.OnBytes)
	if errors.Is(copyErr, io.ErrUnexpectedEOF) {
		copyErr = fmt.Errorf("download incompleto: conexão encerrada após %d bytes: %w", written, copyErr)
	}
	if copyErr == nil && resp.ContentLength >= 0 && written != resp.ContentLength {
		copyErr = fmt.Errorf("download incompleto: %d de %d bytes", written, resp.ContentLength)
	}
	if copyErr == nil {
		copyErr = f.Sync()
	}
	closeErr := f.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil && d.OnBytes != nil {
		d.OnBytes(-written)
	}
	if copyErr != nil {
		// The file was created exclusively by this transfer, so it is ours.
		removeErr := root.Remove(rel)
		if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			copyErr = errors.Join(copyErr, fmt.Errorf("remover parcial: %w", removeErr))
		}
		return DownloadResult{}, TransientNetworkError(ctx, copyErr) && removeErr == nil, copyErr
	}
	return DownloadResult{Status: StatusDownloaded, Path: rel, Bytes: written, ContentType: MediaType(contentType)}, false, nil
}

var errStalled = errors.New("download parado: nenhum dado recebido no tempo limite")

// copyWithStall copies src to dst, cancelling the request when no bytes
// arrive for the stall window.
func copyWithStall(ctx context.Context, cancel context.CancelCauseFunc, dst io.Writer, src io.Reader, stall time.Duration, onBytes func(int64)) (int64, error) {
	timer := time.AfterFunc(stall, func() { cancel(errStalled) })
	defer timer.Stop()
	buf := make([]byte, 64<<10)
	var written int64
	for {
		n, rerr := src.Read(buf)
		if n > 0 {
			timer.Reset(stall)
			m, werr := dst.Write(buf[:n])
			written += int64(m)
			if onBytes != nil && m > 0 {
				onBytes(int64(m))
			}
			if werr != nil {
				return written, werr
			}
			if m != n {
				return written, io.ErrShortWrite
			}
		}
		if errors.Is(rerr, io.EOF) {
			return written, nil
		}
		if rerr != nil {
			if cause := context.Cause(ctx); errors.Is(cause, errStalled) {
				return written, errStalled
			}
			return written, rerr
		}
	}
}
