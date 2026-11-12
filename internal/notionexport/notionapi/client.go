// Package notionapi is the adapter for the unofficial internal API used by
// Notion's web client for public pages (the same endpoints as notion-client /
// react-notion-x). No token or cookie is sent: only pages shared to the web are
// reachable. Every endpoint detail stays in this package.
package notionapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	ne "github.com/matheusvcouto/cli-tools/internal/notionexport"
)

// DefaultBaseURL is the API origin used by current notion-client releases.
const DefaultBaseURL = "https://app.notion.com/api/v3"

const (
	// maxResponseBytes bounds one API response.
	maxResponseBytes = 64 << 20
	// blockBatchSize and signBatchSize bound request bodies.
	blockBatchSize = 100
	signBatchSize  = 50
	// maxMissingRounds bounds the missing-block loop: notion-client loops
	// until nothing is missing, which never ends when Notion withholds a
	// block (private synced block, deleted child). IDs are also requested once.
	maxMissingRounds = 25
	chunkLimit       = 100
)

// ErrNotPublic reports a page Notion did not return to an anonymous client.
var ErrNotPublic = errors.New("página não encontrada ou não pública (verifique se está publicada com \"Compartilhar na web\")")

// Client fetches public pages.
type Client struct {
	HTTP    *http.Client
	BaseURL string
	Retry   ne.RetryPolicy
	// Progress receives human-readable status messages.
	Progress func(string)
	// Warn receives non-fatal problems, e.g. URL signing failures.
	Warn func(string)
}

func (c Client) progress(format string, args ...any) {
	if c.Progress != nil {
		c.Progress(fmt.Sprintf(format, args...))
	}
}

func (c Client) warn(format string, args ...any) {
	if c.Warn != nil {
		c.Warn(fmt.Sprintf(format, args...))
	}
}

// FetchPage implements notionexport.PageSource: it loads the first chunk,
// fetches missing content blocks in bounded rounds and signs reachable file
// URLs so assets can be downloaded.
func (c Client) FetchPage(ctx context.Context, pageID string) (*ne.RecordMap, error) {
	rm, err := c.loadPageChunk(ctx, pageID)
	if err != nil {
		return nil, err
	}
	root, ok := rm.Block(pageID)
	if !ok || root.Type != "page" {
		return nil, ErrNotPublic
	}
	if err := c.fetchMissing(ctx, rm, root); err != nil {
		return nil, err
	}
	c.progress("Recebidos %d blocos.", rm.BlockCount())
	if err := c.signFileURLs(ctx, rm, root); err != nil {
		return nil, err
	}
	return rm, nil
}

func (c Client) loadPageChunk(ctx context.Context, pageID string) (*ne.RecordMap, error) {
	body := map[string]any{"pageId": ne.UUID(pageID), "limit": chunkLimit, "chunkNumber": 0, "cursor": map[string]any{"stack": []any{}}, "verticalColumns": false}
	var resp struct {
		RecordMap json.RawMessage `json:"recordMap"`
	}
	if err := c.post(ctx, "loadPageChunk", body, &resp); err != nil {
		var status *ne.StatusError
		if errors.As(err, &status) && status.StatusCode >= 400 && status.StatusCode < 500 && status.StatusCode != http.StatusTooManyRequests {
			return nil, fmt.Errorf("%w: %v", ErrNotPublic, err)
		}
		return nil, err
	}
	if len(resp.RecordMap) == 0 {
		return nil, ErrNotPublic
	}
	rm, err := ne.DecodeRecordMap(bytes.NewReader(resp.RecordMap))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNotPublic, err)
	}
	return rm, nil
}

// fetchMissing loads referenced blocks absent from the map, like
// notion-utils.getPageContentBlockIds: content, synced-block references,
// link-to-page aliases and page mentions, without entering child pages.
func (c Client) fetchMissing(ctx context.Context, rm *ne.RecordMap, root *ne.Block) error {
	requested := map[string]bool{}
	for round := 0; round < maxMissingRounds; round++ {
		var pending []string
		for _, id := range ContentBlockIDs(rm, root) {
			if !rm.HasBlock(id) && !requested[ne.CompactID(id)] {
				requested[ne.CompactID(id)] = true
				pending = append(pending, id)
			}
		}
		if len(pending) == 0 {
			return nil
		}
		c.progress("Buscando %d bloco(s) adicionais...", len(pending))
		for start := 0; start < len(pending); start += blockBatchSize {
			batch := pending[start:min(start+blockBatchSize, len(pending))]
			requests := make([]map[string]any, len(batch))
			for i, id := range batch {
				requests[i] = map[string]any{"table": "block", "id": ne.UUID(id), "version": -1}
			}
			var resp struct {
				RecordMap struct {
					Block map[string]json.RawMessage `json:"block"`
				} `json:"recordMap"`
			}
			if err := c.post(ctx, "syncRecordValuesMain", map[string]any{"requests": requests}, &resp); err != nil {
				return fmt.Errorf("buscar blocos: %w", err)
			}
			for key, raw := range resp.RecordMap.Block {
				rm.PutBlock(key, raw)
			}
		}
	}
	c.warn("alguns blocos continuam indisponíveis após %d rodadas; o export segue sem eles", maxMissingRounds)
	return nil
}

// ContentBlockIDs lists block IDs that belong to the page content.
func ContentBlockIDs(rm *ne.RecordMap, root *ne.Block) []string {
	seen := map[string]bool{}
	var out []string
	var add func(id string, depth int)
	add = func(id string, depth int) {
		key := ne.CompactID(id)
		if id == "" || seen[key] || depth > 256 {
			return
		}
		seen[key] = true
		out = append(out, id)
		b, ok := rm.Block(id)
		if !ok {
			return
		}
		for _, name := range []string{"transclusion_reference_pointer", "alias_pointer"} {
			if ref := b.FormatPointerID(name); ref != "" {
				add(ref, depth+1)
			}
		}
		for _, raw := range b.Properties {
			for _, seg := range ne.ParseRichText(raw) {
				for _, d := range seg.Decorations {
					if d.Kind == "p" && d.Arg != "" {
						add(d.Arg, depth+1)
					}
				}
			}
		}
		if key != ne.CompactID(root.ID) && (b.Type == "page" || b.Type == "collection_view_page") {
			return
		}
		for _, child := range b.Content {
			add(child, depth+1)
		}
	}
	add(root.ID, 0)
	return out
}

type signRequest struct {
	PermissionRecord struct {
		Table string `json:"table"`
		ID    string `json:"id"`
	} `json:"permissionRecord"`
	URL string `json:"url"`
}

// signFileURLs requests temporary URLs for Notion-hosted files of reachable
// asset blocks. Signing failure is not fatal: those assets keep their source
// URL (and are reported unavailable when it is not downloadable).
func (c Client) signFileURLs(ctx context.Context, rm *ne.RecordMap, root *ne.Block) error {
	var requests []signRequest
	for _, id := range ContentBlockIDs(rm, root) {
		b, ok := rm.Block(id)
		if !ok || !isAssetBlock(b.Type) {
			continue
		}
		source := ne.PlainText(b.Property("source"))
		if !IsNotionFile(source) {
			continue
		}
		var r signRequest
		r.PermissionRecord.Table, r.PermissionRecord.ID, r.URL = "block", ne.UUID(b.ID), source
		requests = append(requests, r)
	}
	if len(requests) == 0 {
		return nil
	}
	c.progress("Assinando %d URL(s) de arquivo...", len(requests))
	for start := 0; start < len(requests); start += signBatchSize {
		batch := requests[start:min(start+signBatchSize, len(requests))]
		var resp struct {
			SignedURLs []string `json:"signedUrls"`
		}
		if err := c.post(ctx, "getSignedFileUrls", map[string]any{"urls": batch}, &resp); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.warn("não foi possível assinar URLs de arquivo: %v", err)
			continue
		}
		for i, r := range batch {
			if i >= len(resp.SignedURLs) || !ne.IsHTTPURL(resp.SignedURLs[i]) {
				continue
			}
			rm.SetSignedURL(r.PermissionRecord.ID, resp.SignedURLs[i])
			rm.SetSignedURL(r.URL, resp.SignedURLs[i])
		}
	}
	return nil
}

func isAssetBlock(t string) bool {
	switch t {
	case "image", "video", "audio", "pdf", "file":
		return true
	}
	return false
}

// IsNotionFile reports sources hosted by Notion that require signing:
// attachment: references, file.notion.so/com, the legacy S3 bucket and the
// image CDN. External URLs (e.g. Unsplash) are fetched as they are.
func IsNotionFile(source string) bool {
	if strings.HasPrefix(source, "attachment:") {
		return true
	}
	u, err := url.Parse(source)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case host == "file.notion.so", host == "file.notion.com", host == "secure.notion-static.com", host == "img.notionusercontent.com":
		return true
	case strings.HasPrefix(host, "prod-files-secure."):
		return true
	case strings.HasSuffix(host, ".amazonaws.com") && strings.HasPrefix(u.Path, "/secure.notion-static.com/"):
		return true
	}
	return false
}

// post sends one JSON request with bounded retries and response size.
func (c Client) post(ctx context.Context, endpoint string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	client := c.HTTP
	if client == nil {
		client = ne.NewHTTPClient()
	}
	target := strings.TrimRight(base, "/") + "/" + endpoint
	return c.Retry.Do(ctx, func(int) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", ne.UserAgent)
		resp, err := client.Do(req)
		if err != nil {
			return ne.TransientNetworkError(ctx, err), fmt.Errorf("%s: %w", endpoint, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			return ne.RetryableStatus(resp.StatusCode), fmt.Errorf("%s: %w", endpoint, ne.NewStatusError(resp))
		}
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		if err != nil {
			return ne.TransientNetworkError(ctx, err), fmt.Errorf("%s: %w", endpoint, err)
		}
		if len(data) > maxResponseBytes {
			return false, fmt.Errorf("%s: resposta excede %d bytes", endpoint, maxResponseBytes)
		}
		if err := json.Unmarshal(data, out); err != nil {
			return false, fmt.Errorf("%s: resposta JSON inválida: %w", endpoint, err)
		}
		return false, nil
	})
}
