package mediacli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

func TestDownloadSummaryFieldsPrivacyAndWidth(t *testing.T) {
	for _, width := range []int{0, 18, 40, 100} {
		var out bytes.Buffer
		inv := &core.Invocation{IO: core.IO{Err: &out}, Terminal: core.Terminal{StderrTTY: width != 0, Width: width}}
		err := printDownloadSummary(inv, mediaget.Info{Title: "Vídeo\x1b\n título"}, mediaget.Source{URL: "https://secret.invalid/token", Referer: "https://secret.invalid/ref", ConcurrentFragments: 25}, mediaget.Selection{Kind: mediaget.Video, Height: 360}, transferEstimate{known: true, bytes: 1024}, "teste", "/synthetic/downloads")
		if err != nil {
			t.Fatal(err)
		}
		got := out.String()
		if strings.Contains(got, "secret.invalid") || strings.Contains(got, "\x1b") {
			t.Fatalf("unsafe summary: %q", got)
		}
		if width == 0 {
			for _, field := range []string{"Resumo do download", "Até 360p", "≈ 1.0 KiB", "Fragmentos paralelos: 25", "Nome base: teste", "Destino: /synthetic/downloads", "Referer: definido"} {
				if !strings.Contains(got, field) {
					t.Errorf("missing %q: %s", field, got)
				}
			}
			if strings.Contains(got, "│") {
				t.Fatal("box in redirected output")
			}
		} else {
			for _, line := range strings.Split(got, "\n") {
				if displayCells(line) > width-1 {
					t.Fatalf("line exceeds %d cells: %q", width-1, line)
				}
			}
		}
	}
}

type failingSummaryWriter struct{}

func (failingSummaryWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestSummaryFailurePreventsDownload(t *testing.T) {
	b := &backend{info: mediaget.Info{Title: "synthetic"}}
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid", "--kind", "video", "--yes", "--output-dir", t.TempDir()}, core.IO{Err: failingSummaryWriter{}})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("error: %v", err)
	}
	if b.downloads != 0 {
		t.Fatal("download started without a readable review")
	}
}

func TestFlagsSkipNamePromptAndReviewBeforeConfirmation(t *testing.T) {
	b := &backend{info: mediaget.Info{Title: "synthetic", Heights: []int{360}}}
	var log bytes.Buffer
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid", "--kind", "video", "--quality", "360", "--concurrent-fragments", "25", "--name", "flag-name", "--output-dir", t.TempDir()}, core.IO{In: strings.NewReader("1\n1\n"), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 || b.last.Name != "flag-name" || b.last.Source.ConcurrentFragments != 25 {
		t.Fatalf("download=%d request=%+v log=%s", b.downloads, b.last, log.String())
	}
	got := log.String()
	if strings.Contains(got, "Nome do arquivo [") {
		t.Fatalf("repeated name prompt: %s", got)
	}
	review, confirmation := strings.Index(got, "Resumo do download"), strings.Index(got, "O que fazer com este download?")
	if review < 0 || confirmation < review {
		t.Fatalf("review missing or late: %s", got)
	}
}

func TestFinalReviewEditsFlagValuesAndRejectsInvalidDestination(t *testing.T) {
	initialDir, finalDir := t.TempDir(), t.TempDir()
	b := &backend{info: mediaget.Info{Title: "Synthetic", Heights: []int{360}}}
	input := "1\n2\n2\nnew-name\n2\n3\n" + initialDir + "/missing\n" + finalDir + "\n2\n4\nfrag\n1\n128\n2\n1\n2\n1\n1\n"
	var log bytes.Buffer
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid", "--kind", "video", "--quality", "360", "--name", "original", "--concurrent-fragments", "25", "--output-dir", initialDir}, core.IO{In: strings.NewReader(input), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 || b.last.Name != "new-name" || b.last.OutputDir != finalDir || b.last.Selection.Kind != mediaget.Audio || b.last.Source.ConcurrentFragments != 128 {
		t.Fatalf("request=%+v downloads=%d log=%s", b.last, b.downloads, log.String())
	}
	if strings.Count(log.String(), "Resumo do download") != 5 {
		t.Fatalf("missing refreshed reviews: %s", log.String())
	}
	if !strings.Contains(log.String(), "Nome base: new-name") || !strings.Contains(log.String(), "Destino: "+finalDir) {
		t.Fatal("review did not reflect edits")
	}
	if b.calls > 8 {
		t.Fatalf("edits repeated metadata queries: %d", b.calls)
	}
}

func TestFinalReviewBackAndCancelDoNotDownload(t *testing.T) {
	b := &backend{info: mediaget.Info{Title: "Synthetic"}}
	var log bytes.Buffer
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid", "--kind", "audio", "--name", "original", "--output-dir", t.TempDir()}, core.IO{In: strings.NewReader("1\n2\n0\n3\n"), Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 0 || strings.Count(log.String(), "Resumo do download") != 2 || !strings.Contains(log.String(), "Cancelado.") {
		t.Fatalf("downloads=%d log=%s", b.downloads, log.String())
	}
}
