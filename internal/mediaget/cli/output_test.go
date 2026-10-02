package mediacli

import (
	"bytes"
	"context"
	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInteractiveSubtitleTXTChoiceAndSummary(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	b := &backend{info: mediaget.Info{Title: "Caption", Tracks: []mediaget.Track{{Lang: "en", Name: "English"}}}}
	var log bytes.Buffer
	// Select subtitle, track, TXT, continue, base name, cancel at final summary.
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid"}, core.IO{In: strings.NewReader("3\n1\n2\n1\ncaption\n3\n"), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil || b.downloads != 0 || !strings.Contains(log.String(), "Tipo: Legenda TXT") || !strings.Contains(log.String(), "Formato da legenda") {
		t.Fatalf("%v %s", err, log.String())
	}
}
func TestMP4FilenameAndFlagReachBackendAndSummary(t *testing.T) {
	for _, args := range [][]string{{"--name", "example.mp4"}, {"--video-format", "mp4", "--name", "example"}} {
		dir := t.TempDir()
		b := &backend{info: mediaget.Info{Title: "Video"}}
		var log bytes.Buffer
		argv := append([]string{"https://example.invalid", "--kind", "video", "--yes", "--output-dir", dir}, args...)
		if err := app(t, b).Run(context.Background(), argv, core.IO{Err: &log}); err != nil {
			t.Fatal(err)
		}
		if b.last.Selection.VideoFormat != "mp4" || b.last.Name != "example" || !strings.Contains(log.String(), "MP4 compatível") {
			t.Fatalf("%+v %s", b.last, log.String())
		}
		if _, err := os.Stat(filepath.Join(dir, "example.mp4")); err != nil {
			t.Fatal(err)
		}
	}
}
func TestOutputFlagsRejectedBeforeMetadata(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	b := &backend{}
	for _, args := range [][]string{{"--kind", "audio", "--video-format", "mp4"}, {"--kind", "video", "--subtitle-format", "txt"}, {"--kind", "subtitle", "--subtitle-format", "bad"}} {
		argv := append([]string{"https://example.invalid", "--yes"}, args...)
		if err := app(t, b).Run(context.Background(), argv, core.IO{}); core.ExitCode(err) != 2 || b.calls != 0 {
			t.Fatalf("%v %d", err, b.calls)
		}
	}
}
