//go:build darwin || linux

package ytdlp

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompatibleMP4CopiesOrRecodesAndValidates(t *testing.T) {
	for _, mode := range []string{"copy", "recode", "video-only", "bad-result", "lost-audio", "failed-encoder", "no-video", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			work := t.TempDir()
			log := filepath.Join(dir, "argv")
			os.WriteFile(filepath.Join(work, "media.mkv"), []byte("source"), 0600)
			initial := `{"streams":[{"codec_type":"video","codec_name":"h264","pix_fmt":"yuv420p"},{"codec_type":"audio","codec_name":"aac"}]}`
			final := initial
			if mode == "recode" {
				initial = `{"streams":[{"codec_type":"video","codec_name":"vp9","pix_fmt":"yuv420p10le"},{"codec_type":"audio","codec_name":"opus"}]}`
			}
			if mode == "video-only" {
				initial = `{"streams":[{"codec_type":"video","codec_name":"h264","pix_fmt":"yuv420p"}]}`
				final = initial
			}
			if mode == "no-video" {
				initial = `{"streams":[{"codec_type":"audio","codec_name":"aac"}]}`
			}
			if mode == "bad-result" {
				final = `{"streams":[{"codec_type":"video","codec_name":"vp9","pix_fmt":"yuv420p"}]}`
			}
			if mode == "lost-audio" {
				final = `{"streams":[{"codec_type":"video","codec_name":"h264","pix_fmt":"yuv420p"}]}`
			}
			executable(t, dir, "ffprobe", `for arg do last="$arg"; done
case "$last" in */compatible.mp4) printf '%s' `+shellQuote(final)+` ;; *) printf '%s' `+shellQuote(initial)+` ;; esac`)
			body := `printf '%s\n' "$@" > ` + shellQuote(log) + `
for arg do last="$arg"; done
printf 'result' > "$last"`
			if mode == "failed-encoder" {
				body = "exit 19"
			}
			if mode == "cancel" {
				body = "/bin/sleep 10"
			}
			ffmpeg := executable(t, dir, "ffmpeg", body)
			a := Adapter{FFmpeg: ffmpeg}
			ctx := context.Background()
			if mode == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 50*time.Millisecond)
				defer cancel()
			}
			err := a.compatibleMP4(ctx, work, nil)
			failed := mode == "bad-result" || mode == "lost-audio" || mode == "failed-encoder" || mode == "no-video" || mode == "cancel"
			if (err != nil) != failed {
				t.Fatalf("%v", err)
			}
			if mode == "cancel" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
			_, sourceErr := os.Stat(filepath.Join(work, "media.mkv"))
			if failed && sourceErr != nil {
				t.Fatal("source removed before validation")
			}
			if !failed {
				if !errors.Is(sourceErr, os.ErrNotExist) {
					t.Fatal("intermediate retained")
				}
				args, _ := os.ReadFile(log)
				text := string(args)
				if !strings.Contains(text, "-map\n0:v:0\n-map\n0:a:0?") || !strings.Contains(text, "-n\n") {
					t.Fatal(text)
				}
				if mode == "recode" {
					if !strings.Contains(text, "libx264") || !strings.Contains(text, "-c:a\naac") || !strings.Contains(text, "yuv420p") {
						t.Fatal(text)
					}
				} else if !strings.Contains(text, "-c:v\ncopy") {
					t.Fatal(text)
				}
			}
		})
	}
}
func TestCompatibleMP4RejectsSymlinksAndMalformedProbe(t *testing.T) {
	dir := t.TempDir()
	work := t.TempDir()
	outside := filepath.Join(t.TempDir(), "important.mp4")
	os.WriteFile(outside, []byte("keep"), 0600)
	if err := os.Symlink(outside, filepath.Join(work, "media.mp4")); err != nil {
		t.Skip(err)
	}
	a := Adapter{}
	if err := a.compatibleMP4(context.Background(), work, nil); err == nil {
		t.Fatal("symlink accepted")
	}
	kept, _ := os.ReadFile(outside)
	if string(kept) != "keep" {
		t.Fatal("outside changed")
	}
	ffmpeg := executable(t, dir, "ffmpeg", "exit 0")
	executable(t, dir, "ffprobe", `printf 'not JSON'`)
	a.FFmpeg = ffmpeg
	if _, err := a.probeVideo(context.Background(), outside); err == nil {
		t.Fatal("bad probe accepted")
	}
}
