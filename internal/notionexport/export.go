// Package notionexport exports public Notion pages to Markdown, HTML and
// local assets. Notion access sits behind PageSource; renderers do no I/O.
package notionexport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

// Format selects the rendered outputs.
type Format string

const (
	FormatMarkdown Format = "md"
	FormatHTML     Format = "html"
	FormatBoth     Format = "both"
)

func (f Format) markdown() bool { return f == FormatMarkdown || f == FormatBoth }
func (f Format) html() bool     { return f == FormatHTML || f == FormatBoth }

// Valid reports a supported format.
func (f Format) Valid() bool { return f == FormatMarkdown || f == FormatHTML || f == FormatBoth }

// PageSource fetches the recordMap of a public page by compact page ID.
type PageSource interface {
	FetchPage(ctx context.Context, pageID string) (*RecordMap, error)
}

// Service coordinates loading, asset transfer, rendering and publication.
type Service struct {
	Source    PageSource
	HTTP      *http.Client
	Retry     RetryPolicy
	Now       func() time.Time
	Generator Generator
	// Progress receives human-readable phase messages (stderr in the CLI).
	Progress func(string)
}

// Page is a loaded page ready to export.
type Page struct {
	RecordMap *RecordMap
	Root      *Block
	PageID    string
	Title     string
	Slug      string
	// Source is the URL/ID given by the user; empty for --from-json.
	Source string
	Assets []Asset
}

func (s Service) progress(format string, args ...any) {
	if s.Progress != nil {
		s.Progress(fmt.Sprintf(format, args...))
	}
}

// Fetch loads a public page from Notion.
func (s Service) Fetch(ctx context.Context, source string) (*Page, error) {
	pageID, err := ParsePageID(source)
	if err != nil {
		return nil, err
	}
	if s.Source == nil {
		return nil, errors.New("fonte do Notion não configurada")
	}
	rm, err := s.Source.FetchPage(ctx, pageID)
	if err != nil {
		return nil, err
	}
	return newPage(rm, pageID, strings.TrimSpace(source))
}

// LoadRecordMap reprocesses a saved page.json. When the manifest.json written
// beside it names a page present in the map, that page is exported; otherwise
// the root is inferred from the map.
func (s Service) LoadRecordMap(path string) (*Page, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrir recordMap: %w", err)
	}
	defer f.Close()
	rm, err := DecodeRecordMap(f)
	if err != nil {
		return nil, err
	}
	pageID := ""
	if mf, err := os.Open(filepath.Join(filepath.Dir(path), "manifest.json")); err == nil {
		_, id, merr := readManifestIdentity(mf)
		_ = mf.Close()
		if b, ok := rm.Block(id); merr == nil && ok && b.Type == "page" {
			pageID = id
		}
	}
	return newPage(rm, pageID, "")
}

func newPage(rm *RecordMap, pageID, source string) (*Page, error) {
	root, err := rm.RootPage(pageID)
	if err != nil {
		return nil, err
	}
	title := PageTitle(root)
	return &Page{RecordMap: rm, Root: root, PageID: CompactID(root.ID), Title: title, Slug: Slugify(title), Source: source, Assets: CollectAssets(rm, root)}, nil
}

// CheckDestination validates the output location read-only, before prompts
// and downloads, so an unusable destination fails early.
func (s Service) CheckDestination(outDir string, page *Page, force bool) error {
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	info, err := os.Lstat(abs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("diretório de saída não é um diretório real: %s", abs)
	}
	root, err := safefs.Open(abs)
	if err != nil {
		return err
	}
	defer root.Close()
	_, err = inspectDestination(root, page.Slug, page.PageID, force)
	return err
}

// ExportOptions controls one export.
type ExportOptions struct {
	OutDir      string
	Format      Format
	Selected    map[string]bool // block IDs to download
	Concurrency int
	Force       bool
	OnDownload  func(DownloadEvent)
}

// Result summarizes a published export.
type Result struct {
	Title           string
	PageID          string
	Dir             string
	MarkdownPath    string
	HTMLPath        string
	ManifestPath    string
	Discovered      int
	Downloaded      int
	Skipped         int
	Failed          int
	DownloadedBytes int64
	// CleanupWarning reports a replaced export that could not be removed.
	CleanupWarning error
}

// Export writes the complete export into a private staging directory beside
// the destination and publishes it only when every file is written. Failure
// or cancellation removes the staging directory and leaves any previous
// export untouched.
func (s Service) Export(ctx context.Context, page *Page, opts ExportOptions) (Result, error) {
	if !opts.Format.Valid() {
		return Result{}, fmt.Errorf("formato inválido: %q", opts.Format)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	outAbs, err := safefs.EnsureDir(opts.OutDir, 0o755)
	if err != nil {
		return Result{}, fmt.Errorf("preparar diretório de saída: %w", err)
	}
	outRoot, err := safefs.Open(outAbs)
	if err != nil {
		return Result{}, err
	}
	defer outRoot.Close()
	if _, err := inspectDestination(outRoot, page.Slug, page.PageID, opts.Force); err != nil {
		return Result{}, err
	}
	suffix, err := randomSuffix()
	if err != nil {
		return Result{}, err
	}
	staging := stagingPrefix + page.Slug + "-" + suffix + ".tmp"
	if err := outRoot.Mkdir(staging, 0o755); err != nil {
		return Result{}, fmt.Errorf("criar staging: %w", err)
	}
	created, err := outRoot.Lstat(staging)
	if err != nil {
		return Result{}, err
	}
	published := false
	defer func() {
		if published {
			return
		}
		// Remove only the directory this export created.
		if current, err := outRoot.Lstat(staging); err == nil && os.SameFile(created, current) {
			_ = outRoot.RemoveAll(staging)
		}
	}()
	stageRoot, err := outRoot.OpenRoot(staging)
	if err != nil {
		return Result{}, err
	}
	result, err := s.writeExport(ctx, stageRoot, page, opts)
	closeErr := stageRoot.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return Result{}, err
	}
	cleanupErr, err := publication{root: outRoot, staging: staging, target: page.Slug, pageID: page.PageID, force: opts.Force}.publish()
	if err != nil {
		return Result{}, err
	}
	published = true
	dir := filepath.Join(outAbs, page.Slug)
	result.Dir = dir
	result.ManifestPath = filepath.Join(dir, "manifest.json")
	if result.MarkdownPath != "" {
		result.MarkdownPath = filepath.Join(dir, result.MarkdownPath)
	}
	if result.HTMLPath != "" {
		result.HTMLPath = filepath.Join(dir, result.HTMLPath)
	}
	result.CleanupWarning = cleanupErr
	return result, nil
}

func (s Service) writeExport(ctx context.Context, root *safefs.Root, page *Page, opts ExportOptions) (Result, error) {
	var selected []Asset
	for _, a := range page.Assets {
		if opts.Selected[a.BlockID] {
			selected = append(selected, a)
		}
	}
	downloads := map[string]DownloadResult{}
	if len(selected) > 0 {
		client := s.HTTP
		if client == nil {
			client = NewHTTPClient()
		}
		downloads = Downloader{Client: client, Retry: s.Retry, Concurrency: opts.Concurrency, Progress: opts.OnDownload}.Download(ctx, root, selected)
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
	}
	resolve := func(blockID, remote string) string {
		if r, ok := downloads[blockID]; ok && r.Status == StatusDownloaded {
			return r.Path
		}
		return remote
	}
	result := Result{Title: page.Title, PageID: page.PageID, Discovered: len(page.Assets)}
	files := ManifestFiles{RecordMap: "page.json", Assets: "assets/"}
	if opts.Format.markdown() {
		s.progress("Gerando Markdown: %s", page.Title)
		if err := writeFile(root, "page.md", []byte(RenderMarkdown(page.RecordMap, page.Root, resolve))); err != nil {
			return Result{}, err
		}
		files.Markdown = ptr("page.md")
		result.MarkdownPath = "page.md"
	}
	if opts.Format.html() {
		s.progress("Gerando HTML: %s", page.Title)
		generator := strings.TrimSpace(s.Generator.Name + " " + s.Generator.Version)
		if err := writeFile(root, "page.html", []byte(RenderHTML(page.RecordMap, page.Root, resolve, generator))); err != nil {
			return Result{}, err
		}
		if err := writeFile(root, "styles.css", []byte(PageCSS)); err != nil {
			return Result{}, err
		}
		files.HTML, files.Styles = ptr("page.html"), ptr("styles.css")
		result.HTMLPath = "page.html"
	}
	recordMap, err := json.MarshalIndent(page.RecordMap, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("serializar recordMap: %w", err)
	}
	if err := writeFile(root, "page.json", append(recordMap, '\n')); err != nil {
		return Result{}, err
	}
	manifest := s.manifest(page, opts, selected, downloads, files)
	result.Downloaded, result.Skipped, result.Failed = manifest.Assets.Downloaded, manifest.Assets.Skipped, manifest.Assets.Failed
	result.DownloadedBytes = manifest.Assets.DownloadedBytes
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Result{}, err
	}
	if err := writeFile(root, "manifest.json", append(data, '\n')); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s Service) manifest(page *Page, opts ExportOptions, selected []Asset, downloads map[string]DownloadResult, files ManifestFiles) Manifest {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	m := Manifest{
		SchemaVersion: ManifestSchemaVersion, Generator: s.Generator, Format: opts.Format,
		Title: page.Title, PageID: page.PageID, ExportedAt: now().UTC().Format("2006-01-02T15:04:05.000Z"),
		BlockCount: page.RecordMap.BlockCount(), Files: files,
	}
	if page.Source != "" {
		m.Source = ptr(page.Source)
	}
	m.Assets.Discovered, m.Assets.Selected = len(page.Assets), len(selected)
	m.Assets.Items = []ManifestItem{}
	for _, a := range page.Assets {
		item := ManifestItem{Index: a.Index, BlockID: a.BlockID, Type: a.Type, Source: a.Source, URL: a.URL, Caption: a.Caption, EstimatedSize: a.EstimatedSize, ContentType: a.ContentType}
		r, ok := downloads[a.BlockID]
		switch {
		case !opts.Selected[a.BlockID]:
			item.Status = StatusSkipped
			m.Assets.Skipped++
		case ok && r.Status == StatusDownloaded:
			item.Status, item.Path, item.Bytes = StatusDownloaded, ptr(r.Path), ptr(r.Bytes)
			if r.ContentType != "" {
				item.ContentType = r.ContentType
			}
			m.Assets.Downloaded++
			m.Assets.DownloadedBytes += r.Bytes
		default:
			msg := "download não retornou resultado"
			if ok && r.Err != nil {
				msg = r.Err.Error()
			}
			item.Status, item.Error = StatusFailed, ptr(msg)
			m.Assets.Failed++
		}
		m.Assets.Items = append(m.Assets.Items, item)
	}
	return m
}

// SelectAll returns every asset of the given types.
func SelectAll(assets []Asset, types map[AssetType]bool) map[string]bool {
	out := map[string]bool{}
	for _, a := range assets {
		if types[a.Type] {
			out[a.BlockID] = true
		}
	}
	return out
}

// writeFile creates name exclusively inside root and syncs it.
func writeFile(root *safefs.Root, name string, data []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("criar %s: %w", name, err)
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("gravar %s: %w", name, err)
	}
	return nil
}

func ptr[T any](v T) *T { return &v }
