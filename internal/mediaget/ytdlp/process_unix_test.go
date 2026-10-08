//go:build darwin || linux

package ytdlp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func executable(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	resolved, err := dependency(name, path)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(dir, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		t.Fatal("fake escaped temporary root")
	}
	actual, err := os.Stat(resolved)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat(path)
	if err != nil || !os.SameFile(expected, actual) {
		t.Fatal("fake identity mismatch")
	}
	return resolved
}
func fixture(t *testing.T) (Adapter, string) {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "argv")
	body := `printf '%s\n' "$@" > ` + shellQuote(log) + `
metadata=no
work=''
want_work=no
for arg do
 if [ "$want_work" = yes ]; then
  case "$arg" in temp:*) ;; *) work="$arg" ;; esac
  want_work=no
 fi
 case "$arg" in --dump-single-json) metadata=yes ;; --paths) want_work=yes ;; esac
done
if [ "$metadata" = yes ]; then
 printf '%s\n' '{"title":"Synthetic","filesize":1234}'
else
 printf '%s' 'synthetic' > "$work/media.mp4"
 printf '%s\n' 'ERROR: synthetic URL https://secret.invalid/?token=TEST' >&2
 printf '%s\n' 'MEDIA_GET_PROGRESS:9|9|NA|1|0' >&2
 printf '%s\n' 'MEDIA_GET_PROCESSING' >&2
fi`
	path := executable(t, dir, "yt-dlp", body)
	ffmpeg := executable(t, dir, "ffmpeg", "exit 0")
	ffprobe := executable(t, dir, "ffprobe", "exit 0")
	return Adapter{Path: path, FFmpeg: ffmpeg, FFprobe: ffprobe}, log
}
func TestRefererAndIsolationAppliedToBothCalls(t *testing.T) {
	a, log := fixture(t)
	src := mediaget.Source{URL: "https://example.invalid/media?a=1&b=2", Referer: "https://origin.invalid/page?x=1&y=2", ConcurrentFragments: 4}
	if _, err := a.Inspect(context.Background(), src, mediaget.Selection{}); err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		for _, arg := range []string{"--ignore-config", "--no-plugin-dirs", "--no-cache-dir", "--no-remote-components", "--referer\n" + src.Referer, "--\n" + src.URL} {
			if !strings.Contains(string(data), arg) {
				t.Fatalf("missing %q in %s", arg, data)
			}
		}
	}
	check()
	work := t.TempDir()
	var events []mediaget.Progress
	req := mediaget.Request{Source: src, Selection: mediaget.Selection{Kind: mediaget.Video, Height: 720}}
	if err := a.Download(context.Background(), req, work, func(p mediaget.Progress) { events = append(events, p) }); err != nil {
		t.Fatal(err)
	}
	check()
	if len(events) != 3 || events[0].Stage != mediaget.Transferring || events[1].Downloaded != 9 || events[2].Stage != mediaget.Processing {
		t.Fatalf("events=%+v", events)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "--concurrent-fragments\n4") || !strings.Contains(string(data), "--abort-on-unavailable-fragments") || !strings.Contains(string(data), "--progress-delta\n0.2") || !strings.Contains(string(data), "bv*[height<=?720]+ba/b[height<=?720]") {
		t.Fatalf("selector: %s", data)
	}
}

func TestBatchOriginHeaderAndProcessingGate(t *testing.T) {
	a, log := fixture(t)
	src := mediaget.Source{URL: "https://example.invalid/video", Origin: "https://origin.invalid"}
	if _, err := a.Inspect(t.Context(), src, mediaget.Selection{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "--add-headers\nOrigin:https://origin.invalid\n") {
		t.Fatal(string(data))
	}
	if err := a.Download(t.Context(), mediaget.Request{Source: src, Selection: mediaget.Selection{Kind: mediaget.Video}}, t.TempDir(), nil); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "--add-headers\nOrigin:https://origin.invalid\n") {
		t.Fatal(string(data))
	}
	a.Path = executable(t, t.TempDir(), "yt-dlp", "exit 0")
	var lastStage mediaget.ProgressStage
	notify := func(p mediaget.Progress) { lastStage = p.Stage }
	want := errors.New("synthetic processing canceled")
	req := mediaget.Request{Source: src, Selection: mediaget.Selection{Kind: mediaget.Video, VideoFormat: "mp4"}, AcquireProcessing: func(context.Context) (func(), error) {
		if lastStage != mediaget.Processing {
			t.Fatal("processing wait has no visible status", lastStage)
		}
		return nil, want
	}}
	if err := a.Download(t.Context(), req, t.TempDir(), notify); !errors.Is(err, want) {
		t.Fatal(err)
	}
	released := false
	req.AcquireProcessing = func(context.Context) (func(), error) { return func() { released = true }, nil }
	// The synthetic probe deliberately fails; even that path releases its slot.
	if err := a.Download(t.Context(), req, t.TempDir(), nil); err == nil || !released {
		t.Fatal(err, released)
	}
}
func TestMissingDependencyBeforeChildStarts(t *testing.T) {
	a, log := fixture(t)
	t.Setenv("PATH", t.TempDir())
	a.FFmpeg = ""
	err := a.Download(context.Background(), mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid"}, Selection: mediaget.Selection{Kind: mediaget.Subtitle}}, t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "ffmpeg") || !strings.Contains(err.Error(), "install") {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(log); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("child spawned before dependency check")
	}
}
func TestMetadataTimeoutKillsChildGroup(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "leaked")
	path := executable(t, dir, "yt-dlp", `(/bin/sleep 0.4; printf 'leaked' > `+shellQuote(marker)+`) &
wait`)
	a := Adapter{Path: path, MetadataTimeout: 50 * time.Millisecond}
	_, err := a.Inspect(context.Background(), mediaget.Source{URL: "https://example.invalid"}, mediaget.Selection{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%v", err)
	}
	time.Sleep(500 * time.Millisecond)
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("descendant survived cancellation")
	}
}
func TestDownloadCancelAndExitAreRedacted(t *testing.T) {
	dir := t.TempDir()
	ffmpeg := executable(t, dir, "ffmpeg", "exit 0")
	path := executable(t, dir, "yt-dlp", "printf '%s\\n' 'https://secret.invalid?token=SYNTHETIC' >&2\nexit 17")
	a := Adapter{Path: path, FFmpeg: ffmpeg}
	req := mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid"}, Selection: mediaget.Selection{Kind: mediaget.Video, Height: 1080}}
	err := a.Download(context.Background(), req, t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "17") || strings.Contains(err.Error(), "SYNTHETIC") {
		t.Fatal(err)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 17 {
		t.Fatalf("exit cause lost: %v", err)
	}
	var process *ProcessError
	if !errors.As(err, &process) || process.ExitCode() != 1 {
		t.Fatalf("CLI execution error class changed: %v", err)
	}
	path = executable(t, dir, "hanging-yt-dlp", "/bin/sleep 10")
	a.Path = path
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := a.Download(ctx, req, t.TempDir(), nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}
func TestChildEnvironmentExcludesUserSecrets(t *testing.T) {
	t.Setenv("SYNTHETIC_TOKEN", "synthetic-secret")
	t.Setenv("PYTHONPATH", "/synthetic/injection")
	t.Setenv("HOME", t.TempDir())
	env := strings.Join(processEnv(), "\n")
	for _, forbidden := range []string{"SYNTHETIC_TOKEN", "PYTHONPATH", "HOME="} {
		if strings.Contains(env, forbidden) {
			t.Fatal(env)
		}
	}
}

func TestAudioAndSubtitleArguments(t *testing.T) {
	for _, selection := range []mediaget.Selection{
		{Kind: mediaget.Audio},
		{Kind: mediaget.Subtitle, Track: mediaget.Track{Lang: "en.orig", Auto: true}},
		{Kind: mediaget.Subtitle, Track: mediaget.Track{Lang: "pt"}},
	} {
		t.Run(string(selection.Kind)+selection.Track.Lang, func(t *testing.T) {
			a, log := fixture(t)
			req := mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid/media"}, Selection: selection}
			if err := a.Download(context.Background(), req, t.TempDir(), nil); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if selection.Kind == mediaget.Audio {
				for _, arg := range []string{"bestaudio/best", "--audio-format\nm4a", "--audio-quality\n256K"} {
					if !strings.Contains(text, arg) {
						t.Fatal(text)
					}
				}
			} else {
				if !strings.Contains(text, "--convert-subs\nsrt") {
					t.Fatal(text)
				}
				if selection.Track.Auto && !strings.Contains(text, "--write-auto-subs") {
					t.Fatal(text)
				}
				if !selection.Track.Auto && !strings.Contains(text, "--write-subs") {
					t.Fatal(text)
				}
				if selection.Track.Lang == "en.orig" && !strings.Contains(text, "^en\\.orig$") {
					t.Fatal("unescaped language regex:", text)
				}
			}
		})
	}
}

func TestAudioProbeMustMatchFFmpegLocationBeforeChildStarts(t *testing.T) {
	a, log := fixture(t)
	other := t.TempDir()
	a.FFprobe = executable(t, other, "ffprobe", "exit 0")
	err := a.Check(context.Background(), mediaget.Selection{Kind: mediaget.Audio})
	if err == nil || !strings.Contains(err.Error(), "ao lado de ffmpeg") {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("child started before coherent tool check")
	}
}

func TestDownloadConcurrencyDefaultAndSerialOverride(t *testing.T) {
	for _, tc := range []struct{ input, want int }{{0, 4}, {1, 1}, {25, 25}, {64, 64}, {128, 128}, {256, 256}} {
		a, log := fixture(t)
		req := mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid", ConcurrentFragments: tc.input}, Selection: mediaget.Selection{Kind: mediaget.Video}}
		if err := a.Download(context.Background(), req, t.TempDir(), nil); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		args := strings.Split(strings.TrimSpace(string(data)), "\n")
		count := 0
		for i, arg := range args {
			if arg == "--concurrent-fragments" {
				count++
				if i+1 == len(args) || args[i+1] != strconv.Itoa(tc.want) {
					t.Fatalf("wrong concurrency: %s", data)
				}
			}
			if arg == "--limit-rate" || arg == "--downloader" || arg == "--no-check-certificates" {
				t.Fatalf("unexpected download policy: %s", data)
			}
		}
		if count != 1 {
			t.Fatalf("concurrency supplied %d times: %s", count, data)
		}
	}
}
