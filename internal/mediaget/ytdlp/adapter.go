// Package ytdlp adapts system yt-dlp/FFmpeg without shell or user configuration.
package ytdlp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type Adapter struct {
	// Optional explicit paths primarily support isolated process tests.
	Path, FFmpeg, FFprobe string
	MetadataTimeout       time.Duration
}

type DependencyError struct {
	Tool  string
	Cause error
}

func (e *DependencyError) Error() string {
	return fmt.Sprintf("dependência %s ausente ou não executável; instale com: %s", e.Tool, installHint(e.Tool))
}
func (e *DependencyError) Unwrap() error { return e.Cause }
func (e *DependencyError) ExitCode() int { return 3 }

func dependency(name, explicit string) (string, error) {
	candidate := name
	if explicit != "" {
		candidate = explicit
	}
	path, err := exec.LookPath(candidate)
	if err != nil {
		return "", &DependencyError{Tool: name, Cause: err}
	}
	return path, nil
}
func (a Adapter) Check(ctx context.Context, sel mediaget.Selection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := platformAvailable(); err != nil {
		return err
	}
	if _, err := dependency("yt-dlp", a.Path); err != nil {
		return err
	}
	if sel.Kind == "" {
		return nil
	} // Inspection uses only yt-dlp.
	if _, err := dependency("ffmpeg", a.FFmpeg); err != nil {
		return err
	}
	if sel.Kind == mediaget.Audio {
		if _, err := dependency("ffprobe", a.FFprobe); err != nil {
			return err
		}
	}
	return nil
}
func baseArgs(src mediaget.Source) []string {
	// netrc is opt-in in the official parser; there is no --no-netrc flag.
	args := []string{"--ignore-config", "--no-plugin-dirs", "--no-cache-dir", "--no-remote-components", "--no-playlist", "--playlist-items", "1", "--color", "never", "--socket-timeout", "20"}
	if src.Referer != "" {
		args = append(args, "--referer", src.Referer)
	}
	return args
}
func format(sel mediaget.Selection) string {
	if sel.Kind == mediaget.Audio {
		return "bestaudio/best"
	}
	if sel.Kind != mediaget.Video {
		return ""
	}
	if sel.Height == 0 {
		return "bv*+ba/b"
	}
	// Unknown heights remain eligible; known heights above the limit do not.
	return fmt.Sprintf("bv*[height<=?%d]+ba/b[height<=?%d]", sel.Height, sel.Height)
}
func (a Adapter) Inspect(ctx context.Context, src mediaget.Source, sel mediaget.Selection) (mediaget.Info, error) {
	if err := mediaget.ValidateSource(src); err != nil {
		return mediaget.Info{}, err
	}
	if err := a.Check(ctx, mediaget.Selection{}); err != nil {
		return mediaget.Info{}, err
	}
	path, err := dependency("yt-dlp", a.Path)
	if err != nil {
		return mediaget.Info{}, err
	}
	timeout := a.MetadataTimeout
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := append(baseArgs(src), "--dump-single-json", "--skip-download")
	if f := format(sel); f != "" {
		args = append(args, "-f", f)
	}
	args = append(args, "--", src.URL)
	cmd := command(ctx, path, args)
	var out bytes.Buffer
	cmd.Stdout = &boundedWriter{w: &out, remaining: 8 << 20}
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return mediaget.Info{}, processError(ctx, err, "consultar metadados")
	}
	return decodeInfo(out.Bytes())
}

func (a Adapter) Download(ctx context.Context, req mediaget.Request, work string, progress func(mediaget.Progress)) error {
	if err := a.Check(ctx, req.Selection); err != nil {
		return err
	}
	path, err := dependency("yt-dlp", a.Path)
	if err != nil {
		return err
	}
	args := append(baseArgs(req.Source), "--newline", "--progress", "--progress-delta", "1", "--progress-template", "download:MEDIA_GET_PROGRESS:%(progress.downloaded_bytes)s|%(progress.total_bytes)s|%(progress.total_bytes_estimate)s|%(progress.speed)s|%(progress.eta)s", "--progress-template", "postprocess:MEDIA_GET_PROCESSING", "--paths", work, "--paths", "temp:"+work, "-o", "media.%(ext)s", "--no-overwrites")
	// Let yt-dlp locate both ffmpeg and ffprobe in the explicitly resolved
	// ffmpeg directory; fixtures use the same layout as system packages.
	ffmpeg, err := dependency("ffmpeg", a.FFmpeg)
	if err != nil {
		return err
	}
	args = append(args, "--ffmpeg-location", ffmpeg)
	switch req.Selection.Kind {
	case mediaget.Video:
		args = append(args, "-f", format(req.Selection), "--merge-output-format", "mp4/mkv")
	case mediaget.Audio:
		args = append(args, "-f", format(req.Selection), "-x", "--audio-format", "m4a", "--audio-quality", "256K")
	case mediaget.Subtitle:
		args = append(args, "--skip-download", "--sub-format", "srt/best", "--sub-langs", "^"+regexp.QuoteMeta(req.Selection.Track.Lang)+"$", "--convert-subs", "srt")
		if req.Selection.Track.Auto {
			args = append(args, "--write-auto-subs")
		} else {
			args = append(args, "--write-subs")
		}
	default:
		return errors.New("tipo de download inválido")
	}
	args = append(args, "--", req.Source.URL)
	if progress != nil {
		progress(mediaget.Progress{Stage: mediaget.Transferring})
	}
	cmd := command(ctx, path, args)
	output := &progressWriter{notify: progress}
	cmd.Stdout = output
	cmd.Stderr = output
	if err := cmd.Run(); err != nil {
		return processError(ctx, err, "baixar mídia")
	}
	return nil
}

func processEnv() []string {
	var out []string
	for _, key := range []string{"PATH", "LANG", "LC_ALL"} {
		if value, ok := os.LookupEnv(key); ok {
			out = append(out, key+"="+value)
		}
	}
	return out
}

// ProcessError retains the cause without exposing child stderr or signed URLs.
type ProcessError struct {
	Action string
	Cause  *exec.ExitError
}

func (e *ProcessError) Error() string {
	return fmt.Sprintf("yt-dlp falhou ao %s (código %d); confira URL, Referer, acesso ao site e versão instalada", e.Action, e.Cause.ExitCode())
}
func (e *ProcessError) Unwrap() error { return e.Cause }

// Preserve the CLI execution-error class while retaining the child status.
func (e *ProcessError) ExitCode() int { return 1 }

func processError(ctx context.Context, err error, action string) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s interrompido: %w", action, ctx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return &ProcessError{Action: action, Cause: exit}
	}
	return fmt.Errorf("yt-dlp falhou ao %s: %w", action, err)
}

type boundedWriter struct {
	w         io.Writer
	remaining int
}

func (b *boundedWriter) Write(p []byte) (int, error) {
	if len(p) > b.remaining {
		return 0, errors.New("metadados excedem 8 MiB")
	}
	n, err := b.w.Write(p)
	b.remaining -= n
	return n, err
}

// Only numerical fields from our progress template reach the UI. Raw child
// output may contain signed URLs, Referer or terminal escapes and is discarded.
type progressWriter struct {
	mu     sync.Mutex
	line   []byte
	notify func(mediaget.Progress)
}

func (p *progressWriter) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, b := range data {
		if b == '\n' || b == '\r' {
			if event, ok := parseProgress(string(p.line)); ok && p.notify != nil {
				p.notify(event)
			}
			p.line = p.line[:0]
		} else if len(p.line) < 2048 {
			p.line = append(p.line, b)
		}
	}
	return len(data), nil
}
func parseProgress(line string) (mediaget.Progress, bool) {
	if line == "MEDIA_GET_PROCESSING" {
		return mediaget.Progress{Stage: mediaget.Processing}, true
	}
	if !strings.HasPrefix(line, "MEDIA_GET_PROGRESS:") {
		return mediaget.Progress{}, false
	}
	fields := strings.Split(strings.TrimPrefix(line, "MEDIA_GET_PROGRESS:"), "|")
	if len(fields) != 5 {
		return mediaget.Progress{}, false
	}
	downloaded, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil || downloaded < 0 {
		return mediaget.Progress{}, false
	}
	total, _ := strconv.ParseInt(fields[1], 10, 64)
	if total <= 0 {
		total, _ = strconv.ParseInt(fields[2], 10, 64)
	}
	if total < 0 {
		total = 0
	}
	speed, _ := strconv.ParseFloat(fields[3], 64)
	if speed < 0 || speed > 1e15 || math.IsNaN(speed) || math.IsInf(speed, 0) {
		speed = 0
	}
	eta, _ := strconv.Atoi(fields[4])
	if eta < 0 || eta > 365*24*60*60 {
		eta = 0
	}
	return mediaget.Progress{Stage: mediaget.Transferring, Downloaded: downloaded, Total: total, Speed: speed, ETA: eta}, true
}

type rawFormat struct {
	Size    float64 `json:"filesize"`
	Approx  float64 `json:"filesize_approx"`
	Bitrate float64 `json:"tbr"`
	Height  int     `json:"height"`
}
type rawTrack struct {
	Name string `json:"name"`
}

func decodeInfo(data []byte) (mediaget.Info, error) {
	var raw struct {
		Type     string          `json:"_type"`
		Entries  json.RawMessage `json:"entries"`
		Title    string          `json:"title"`
		Duration float64         `json:"duration"`
		rawFormat
		Parts     []rawFormat           `json:"requested_formats"`
		Formats   []rawFormat           `json:"formats"`
		Subtitles map[string][]rawTrack `json:"subtitles"`
		Automatic map[string][]rawTrack `json:"automatic_captions"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return mediaget.Info{}, errors.New("yt-dlp retornou metadados inválidos")
	}
	if raw.Type == "playlist" || raw.Type == "multi_video" || len(raw.Entries) > 0 && string(raw.Entries) != "null" {
		return mediaget.Info{}, errors.New("playlists e coleções ainda não são suportadas; informe a URL de uma mídia individual")
	}
	size := func(f rawFormat) mediaget.FormatSize {
		return mediaget.FormatSize{Bytes: f.Size, ApproxBytes: f.Approx, Bitrate: f.Bitrate}
	}
	info := mediaget.Info{Title: raw.Title, Duration: raw.Duration, Size: size(raw.rawFormat)}
	for _, f := range raw.Parts {
		info.Parts = append(info.Parts, size(f))
	}
	heights := map[int]bool{}
	for _, f := range raw.Formats {
		if f.Height > 0 {
			heights[f.Height] = true
		}
	}
	for height := range heights {
		info.Heights = append(info.Heights, height)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(info.Heights)))
	add := func(lang string, tracks []rawTrack, auto bool) {
		if !mediaget.ValidTrackLang(lang) {
			return
		}
		name := lang
		for _, track := range tracks {
			if track.Name != "" {
				name = track.Name
				break
			}
		}
		info.Tracks = append(info.Tracks, mediaget.Track{Lang: lang, Name: name, Auto: auto})
	}
	for lang, tracks := range raw.Subtitles {
		if len(tracks) > 0 {
			add(lang, tracks, false)
		}
	}
	for lang, tracks := range raw.Automatic {
		if len(raw.Subtitles[lang]) == 0 && len(tracks) > 0 {
			add(lang, tracks, true)
		}
	}
	sort.Slice(info.Tracks, func(i, j int) bool { return info.Tracks[i].Lang < info.Tracks[j].Lang })
	return info, nil
}
