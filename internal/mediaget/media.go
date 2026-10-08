// Package mediaget owns selection, estimates and safe publication of media.
package mediaget

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/matheusvcouto/cli-tools/internal/safefs"
)

type Kind string

const (
	Video    Kind = "video"
	Audio    Kind = "audio"
	Subtitle Kind = "subtitle"
)

const DefaultConcurrentFragments = 4
const MaxConcurrentFragments = 256

type Source struct {
	URL, Referer, Origin string
	ConcurrentFragments  int
}
type Track struct {
	Lang, Name string
	Auto       bool
}
type Selection struct {
	Kind           Kind
	Height         int
	Track          Track
	SubtitleFormat string // Empty means SRT.
	VideoFormat    string // Empty/auto preserves the backend container; mp4 requests H.264/AAC.
}
type FormatSize struct {
	Bytes, ApproxBytes, Bitrate float64
	StreamID                    string // Opaque backend identity; never render source identifiers.
}
type Info struct {
	Title              string
	Duration           float64
	Size               FormatSize
	Parts              []FormatSize
	Heights            []int
	Tracks             []Track
	UnknownVideoHeight bool
}
type ProgressStage string

const (
	Transferring ProgressStage = "transferring"
	Processing   ProgressStage = "processing"
	Publishing   ProgressStage = "publishing"
)

type Progress struct {
	Stage             ProgressStage
	Downloaded, Total int64
	Speed             float64
	ETA               int
	Estimated         bool
	StreamID          string
}
type Request struct {
	Source          Source
	Selection       Selection
	OutputDir, Name string
	KeepIncomplete  bool // Caller takes responsibility for offering retention or discard after failure.
	// AcquireProcessing optionally limits the adapter's extra conversion stage.
	// Transfer/merge performed by the downloader remains bounded by batch jobs.
	AcquireProcessing func(context.Context) (func(), error)
}
type Result struct {
	Path           string
	Bytes          int64
	CleanupWarning error
	Incomplete     *Incomplete
}

// Backend is the boundary for extraction and transfer. It does not publish the
// final file or decide the destination. Other extractors can implement it.
// Check/Inspect/Download must support concurrent calls and respect context
// cancellation. Download receives a distinct private working directory per call.
type Backend interface {
	Check(context.Context, Selection) error
	Inspect(context.Context, Source, Selection) (Info, error)
	Download(context.Context, Request, string, func(Progress)) error
}
type Service struct{ Backend Backend }

// FragmentConcurrency resolves the zero-value source to the bounded default.
func (src Source) FragmentConcurrency() int {
	if src.ConcurrentFragments == 0 {
		return DefaultConcurrentFragments
	}
	return src.ConcurrentFragments
}

func ValidateSource(src Source) error {
	if src.FragmentConcurrency() < 1 || src.FragmentConcurrency() > MaxConcurrentFragments {
		return fmt.Errorf("fragmentos paralelos devem estar entre 1 e %d", MaxConcurrentFragments)
	}
	if err := validateURL(src.URL); err != nil {
		return err
	}
	if src.Referer != "" {
		if err := validateURL(src.Referer); err != nil {
			return fmt.Errorf("Referer: %w", err)
		}
	}
	if src.Origin != "" {
		if err := validateURL(src.Origin); err != nil {
			return errors.New("Origin deve ser uma origem HTTP(S) válida")
		}
		u, _ := url.Parse(src.Origin)
		if u.Path != "" && u.Path != "/" || strings.ContainsAny(src.Origin, "?#") {
			return errors.New("Origin não pode conter path, query ou fragmento")
		}
	}
	return nil
}
func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return errors.New("informe uma URL http(s) válida, sem credenciais embutidas")
	}
	return nil
}
func (s Service) Inspect(ctx context.Context, src Source, selection Selection) (Info, error) {
	if err := ValidateSource(src); err != nil {
		return Info{}, err
	}
	if s.Backend == nil {
		return Info{}, errors.New("backend indisponível")
	}
	return s.Backend.Inspect(ctx, src, selection)
}
func DefaultDownloadDir(home string) string { return filepath.Join(home, "Downloads") }

func (s Service) Check(ctx context.Context, selection Selection) error {
	if s.Backend == nil {
		return errors.New("backend indisponível")
	}
	return s.Backend.Check(ctx, selection)
}

// ValidateDestination is read-only. Missing suffixes may be created when the
// confirmed transfer begins; existing files/symlinks are never replaced.
func (s Service) ValidateDestination(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("diretório de download vazio")
	}
	ancestor, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for {
		_, err = os.Lstat(ancestor)
		if err == nil {
			root, err := safefs.Open(ancestor)
			if err != nil {
				return fmt.Errorf("destino deve ser uma pasta real ou estar abaixo de uma: %w", err)
			}
			return root.Close()
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("diretório de download inválido: %w", err)
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return errors.New("destino sem pasta existente para criação")
		}
		ancestor = parent
	}
}

// SafeName caps bytes (rather than runes), leaving room for suffix and extension.
func SafeName(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsControl(r) {
			continue
		}
		if strings.ContainsRune(`/\:*?"<>|`, r) {
			r = '-'
		}
		if b.Len()+utf8.RuneLen(r) > 180 {
			break
		}
		b.WriteRune(r)
	}
	name := strings.Trim(b.String(), " .")
	if name == "" {
		return "media"
	}
	return name
}
func positive(values ...float64) float64 {
	for _, n := range values {
		if n > 0 && !math.IsNaN(n) && !math.IsInf(n, 0) {
			return n
		}
	}
	return 0
}
func sizeOf(f FormatSize, duration float64) float64 {
	if n := positive(f.Bytes, f.ApproxBytes); n > 0 {
		return n
	}
	return positive(duration) * positive(f.Bitrate) * 1000 / 8
}

// EstimatedSize refers to bytes transferred, before merging or transcoding.
// A missing part makes the whole estimate unknown; it is never silently zero.
func EstimatedSize(info Info) (int64, bool) {
	n := sizeOf(info.Size, info.Duration)
	if len(info.Parts) > 0 {
		n = 0
		for _, f := range info.Parts {
			part := sizeOf(f, info.Duration)
			if part <= 0 {
				return 0, false
			}
			n += part
		}
	}
	if n < 1 || n >= float64(math.MaxInt64) || math.IsNaN(n) || math.IsInf(n, 0) {
		return 0, false
	}
	return int64(n), true
}
func HumanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, unit := range []string{"KiB", "MiB", "GiB", "TiB"} {
		value /= 1024
		if value < 1024 || unit == "TiB" {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return ""
}
func validateSelection(sel Selection) error {
	if sel.SubtitleFormat != "" && (sel.Kind != Subtitle || sel.SubtitleFormat != "srt" && sel.SubtitleFormat != "txt") {
		return errors.New("formato de legenda inválido; use srt ou txt")
	}
	if sel.VideoFormat != "" && (sel.Kind != Video || sel.VideoFormat != "auto" && sel.VideoFormat != "mp4") {
		return errors.New("formato de vídeo inválido; use auto ou mp4")
	}
	switch sel.Kind {
	case Video:
		if sel.Height != 0 && sel.Height != 360 && sel.Height != 480 && sel.Height != 720 && sel.Height != 1080 && sel.Height != 2160 {
			return errors.New("qualidade inválida")
		}
	case Audio:
	case Subtitle:
		if !ValidTrackLang(sel.Track.Lang) {
			return errors.New("selecione uma faixa de legenda com identificador válido")
		}
	default:
		return errors.New("tipo de download inválido")
	}
	return nil
}

// ValidateSelection checks a resolved request before metadata or mutation.
func ValidateSelection(sel Selection) error { return validateSelection(sel) }

// ValidTrackLang permits extractor language identifiers but rejects path,
// template and comma-list syntax before it can affect output filenames.
func ValidTrackLang(lang string) bool {
	if lang == "" || lang == "." || lang == ".." || len(lang) > 80 {
		return false
	}
	for _, r := range lang {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func (s Service) Download(ctx context.Context, req Request, progress func(Progress)) (result Result, err error) {
	if strings.TrimSpace(req.OutputDir) == "" {
		return result, errors.New("diretório de download não informado")
	}
	if err = ValidateSource(req.Source); err != nil {
		return
	}
	req.Name, req.Selection = OutputSelection(req.Name, req.Selection)
	if err = validateSelection(req.Selection); err != nil {
		return
	}
	if s.Backend == nil {
		return result, errors.New("backend indisponível")
	}
	if err = s.Backend.Check(ctx, req.Selection); err != nil {
		return
	}
	if err = ctx.Err(); err != nil {
		return
	}
	req.OutputDir, err = safefs.EnsureDir(req.OutputDir, 0755)
	if err != nil {
		return
	}
	root, err := safefs.Open(req.OutputDir)
	if err != nil {
		return result, fmt.Errorf("diretório de download inválido: %w", err)
	}
	defer root.Close()
	// Random, private working directory in the destination filesystem. All
	// material is created here, not moved from a system temporary directory.
	workName := "Media Get — Incompletos-" + rand.Text()
	if err = root.Mkdir(workName, 0700); err != nil {
		return
	}
	work := filepath.Join(req.OutputDir, workName)
	original, err := root.Lstat(workName)
	if err != nil {
		return
	}
	sameWork := func() error {
		current, e := root.Lstat(workName)
		if e != nil {
			return e
		}
		byPath, e := os.Lstat(work)
		if e != nil {
			return e
		}
		if !current.IsDir() || !byPath.IsDir() || !os.SameFile(original, current) || !os.SameFile(current, byPath) {
			return errors.New("área de download mudou durante a operação")
		}
		return nil
	}
	defer func() {
		if e := sameWork(); e != nil {
			result.CleanupWarning = e
			if result.Path == "" {
				err = fmt.Errorf("%w; limpeza bloqueada: %v; verifique %s", err, e, work)
			}
			return
		}
		if result.Path == "" && req.KeepIncomplete {
			parent, e := root.Lstat(".")
			if e != nil {
				err = errors.Join(err, e)
				return
			}
			result.Incomplete = &Incomplete{Path: work, parent: req.OutputDir, name: workName, original: original, parentIdentity: parent}
			return
		}
		result.CleanupWarning = root.RemoveAll(workName)
		if result.Path == "" {
			if result.CleanupWarning != nil {
				err = fmt.Errorf("%w; não foi possível descartar incompletos em %s: %v", err, work, result.CleanupWarning)
			} else {
				err = fmt.Errorf("%w; arquivos incompletos descartados", err)
			}
		}
	}()

	if err = sameWork(); err != nil {
		return
	}
	if err = s.Backend.Download(ctx, req, work, progress); err != nil {
		return
	}
	if err = ctx.Err(); err != nil {
		return
	}
	if progress != nil {
		progress(Progress{Stage: Publishing})
	}
	if err = sameWork(); err != nil {
		return
	}
	dir, err := root.Open(workName)
	if err != nil {
		return
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if err = errors.Join(readErr, closeErr); err != nil {
		return
	}
	if len(entries) != 1 || !entries[0].Type().IsRegular() {
		return result, errors.New("download não produziu exatamente um arquivo regular")
	}
	sourceName := filepath.Join(workName, entries[0].Name())
	file, err := root.Lstat(sourceName)
	if err != nil {
		return
	}
	if !file.Mode().IsRegular() || file.Size() == 0 {
		return result, errors.New("arquivo baixado vazio ou inválido")
	}
	ext := strings.ToLower(filepath.Ext(entries[0].Name()))
	allowed := map[string]bool{".mp4": true, ".mkv": true, ".webm": true, ".mov": true, ".flv": true, ".avi": true, ".ts": true}
	if req.Selection.Kind == Audio {
		allowed = map[string]bool{".m4a": true}
	}
	if req.Selection.Kind == Subtitle {
		allowed = map[string]bool{".srt": true}
	}
	if !allowed[ext] {
		return result, errors.New("formato de saída inesperado")
	}
	if req.Selection.Kind == Video && req.Selection.VideoFormat == "mp4" && ext != ".mp4" {
		return result, errors.New("backend não produziu o MP4 solicitado")
	}
	if req.Selection.Kind == Subtitle && req.Selection.SubtitleFormat == "txt" {
		sourceName, file, err = subtitleText(root, sourceName, workName)
		if err != nil {
			return
		}
		ext = ".txt"
	}
	base := SafeName(req.Name)
	for n := 0; n < 10000; n++ {
		if err = ctx.Err(); err != nil {
			return
		}
		name := base + ext
		if n > 0 {
			name = fmt.Sprintf("%s (%d)%s", base, n, ext)
		}
		err = root.Link(sourceName, name) // create-exclusive publication; no replace.
		if err == nil {
			return Result{Path: filepath.Join(req.OutputDir, name), Bytes: file.Size()}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return result, fmt.Errorf("publicar download sem substituir arquivo: %w", err)
		}
	}
	return result, errors.New("muitas colisões de nome no destino")
}

// OutputSelection interprets supported explicit extensions as output requests,
// not as part of the base name. It is applied before summary and before transfer.
func OutputSelection(name string, sel Selection) (string, Selection) {
	ext := strings.ToLower(filepath.Ext(name))
	if sel.Kind == Video && ext == ".mp4" {
		sel.VideoFormat = "mp4"
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if sel.Kind == Subtitle && (ext == ".txt" || ext == ".srt") {
		sel.SubtitleFormat = ext[1:]
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return name, sel
}
