package mediacli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type backend struct {
	mu               sync.Mutex
	calls, downloads int
	estimateErr      error
	last             mediaget.Request
	info             mediaget.Info
	sources          []mediaget.Source
}

func (b *backend) Check(context.Context, mediaget.Selection) error { return nil }
func (b *backend) Inspect(_ context.Context, src mediaget.Source, sel mediaget.Selection) (mediaget.Info, error) {
	b.mu.Lock()
	b.calls++
	b.sources = append(b.sources, src)
	b.mu.Unlock()
	if sel.Kind != "" && b.estimateErr != nil {
		return mediaget.Info{}, b.estimateErr
	}
	return b.info, nil
}
func (b *backend) Download(_ context.Context, r mediaget.Request, d string, _ func(mediaget.Progress)) error {
	b.downloads++
	b.last = r
	ext := "mp4"
	if r.Selection.Kind == mediaget.Audio {
		ext = "m4a"
	}
	if r.Selection.Kind == mediaget.Subtitle {
		ext = "srt"
	}
	return os.WriteFile(filepath.Join(d, "media."+ext), []byte("synthetic"), 0600)
}
func app(t *testing.T, b *backend) *core.CompiledApp {
	t.Helper()
	a, err := New(mediaget.Service{Backend: b}, core.ProductMetadata{Version: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func TestStaticEndpointsDoNotReadHomeOrBackend(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv(DownloadEnv, "")
	a, err := New(mediaget.Service{}, core.ProductMetadata{Version: "0.1.0"})
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"--help"}, {"--version"}, {"version", "--json"}, {"__cli", "schema"}, {"__cli", "contract"}, {"completion", "generate", "zsh"}} {
		var out bytes.Buffer
		if err := a.Run(context.Background(), args, core.IO{Out: &out}); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if out.Len() == 0 {
			t.Fatalf("%v empty", args)
		}
	}
}
func TestContractLockIsCurrent(t *testing.T) {
	a, err := New(mediaget.Service{}, core.ProductMetadata{Version: "0.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.ContractJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../cmd/media-get/cli.contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("media-get contract lock is stale")
	}
}
func TestNonInteractiveFailsBeforeNetwork(t *testing.T) {
	b := &backend{}
	a := app(t, b)
	t.Setenv(DownloadEnv, t.TempDir())
	err := a.Run(context.Background(), []string{"https://example.invalid"}, core.IO{})
	if core.ExitCode(err) != 3 || b.calls != 0 {
		t.Fatalf("%v calls %d", err, b.calls)
	}
	for _, args := range [][]string{{"--yes"}, {"https://example.invalid", "--yes"}, {"https://example.invalid", "--kind", "audio", "--quality", "720", "--yes"}} {
		if err := a.Run(context.Background(), args, core.IO{}); core.ExitCode(err) != 2 {
			t.Fatalf("%v => %v", args, err)
		}
	}
}
func TestAutomationOutputPrecedenceAndUnknownEstimate(t *testing.T) {
	envDir := t.TempDir()
	cliDir := t.TempDir()
	t.Setenv(DownloadEnv, envDir)
	b := &backend{info: mediaget.Info{Title: "Example"}, estimateErr: errors.New("size probe failed")}
	a := app(t, b)
	var out, log bytes.Buffer
	err := a.Run(context.Background(), []string{"https://example.invalid/media", "--kind", "audio", "--yes", "--referer", "https://origin.invalid", "--output-dir", cliDir}, core.IO{Out: &out, Err: &log})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 || b.last.Source.Referer != "https://origin.invalid" || b.last.OutputDir != cliDir {
		t.Fatalf("%+v", b.last)
	}
	if !strings.Contains(log.String(), "Não foi possível calcular") || !strings.Contains(out.String(), cliDir) {
		t.Fatalf("%s %s", log.String(), out.String())
	}
	entries, _ := os.ReadDir(envDir)
	if len(entries) != 0 {
		t.Fatal("wrote env directory instead of CLI override")
	}
}
func TestDefaultHomeDownloadsResolution(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "Downloads")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// A missing env key must fall through; an explicitly empty key is an error.
	prior, present := os.LookupEnv(DownloadEnv)
	os.Unsetenv(DownloadEnv)
	t.Cleanup(func() {
		if present {
			os.Setenv(DownloadEnv, prior)
		} else {
			os.Unsetenv(DownloadEnv)
		}
	})
	b := &backend{info: mediaget.Info{Title: "Default"}}
	err := app(t, b).Run(context.Background(), []string{"https://example.invalid", "--kind", "audio", "--yes"}, core.IO{})
	if err != nil || b.last.OutputDir != dir {
		t.Fatalf("%v %+v", err, b.last)
	}
}
func TestInteractiveRefererDefaultAndBack(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(DownloadEnv, dir)
	b := &backend{info: mediaget.Info{Title: "Wizard"}}
	// URL, video, back, audio, continue, default name, confirm.
	input := "https://example.invalid/media\n1\n0\n2\n1\n\n1\n"
	var out, log bytes.Buffer
	err := app(t, b).Run(context.Background(), nil, core.IO{In: strings.NewReader(input), Out: &out, Err: &log, Terminal: core.Terminal{StdinTTY: true}})
	if err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 || b.last.Selection.Kind != mediaget.Audio || b.last.Source.Referer != "" {
		t.Fatalf("%+v", b.last)
	}
}
func TestSubtitleSelectionAndDecline(t *testing.T) {
	t.Setenv(DownloadEnv, t.TempDir())
	b := &backend{info: mediaget.Info{Title: "Caption", Tracks: []mediaget.Track{{Lang: "pt", Name: "Português"}}}}
	a := app(t, b)
	if err := a.Run(context.Background(), []string{"https://example.invalid", "--kind", "subtitle", "--subtitle-lang", "pt", "--yes"}, core.IO{}); err != nil {
		t.Fatal(err)
	}
	if b.last.Selection.Track.Lang != "pt" {
		t.Fatal(b.last)
	}
	input := "3\n1\n1\n\n3\n" // Subtitle, track, continue, name, decline.
	if err := a.Run(context.Background(), []string{"https://example.invalid"}, core.IO{In: strings.NewReader(input), Terminal: core.Terminal{StdinTTY: true}}); err != nil {
		t.Fatal(err)
	}
	if b.downloads != 1 {
		t.Fatal("decline downloaded")
	}
}

type blockedInteraction struct{ started, release chan struct{} }

func (b blockedInteraction) Text(context.Context, core.Prompt) (string, error) {
	close(b.started)
	<-b.release
	return "", nil
}
func (b blockedInteraction) Confirm(context.Context, core.Prompt) (bool, error)  { return false, nil }
func (b blockedInteraction) Secret(context.Context, core.Prompt) (string, error) { return "", nil }
func TestPromptReturnsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := blockedInteraction{started: make(chan struct{}), release: make(chan struct{})}
	defer close(b.release)
	inv := &core.Invocation{Context: ctx, Interaction: b}
	done := make(chan error, 1)
	go func() { _, err := ask(inv, "URL", ""); done <- err }()
	<-b.started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("prompt ignored cancellation")
	}
}
func TestEnvironmentDirectoryAndTypedQuality(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(DownloadEnv, dir)
	b := &backend{info: mediaget.Info{Title: "Env"}}
	a := app(t, b)
	if err := a.Run(context.Background(), []string{"https://example.invalid", "--kind", "video", "--quality", "720", "--yes"}, core.IO{}); err != nil {
		t.Fatal(err)
	}
	if b.last.OutputDir != dir || b.last.Selection.Height != 720 {
		t.Fatalf("%+v", b.last)
	}
	t.Setenv(DownloadEnv, "")
	if err := a.Run(context.Background(), []string{"https://example.invalid", "--kind", "audio", "--yes"}, core.IO{}); core.ExitCode(err) != 2 {
		t.Fatalf("empty env: %v", err)
	}
}

func TestAutomationFragmentConcurrencyDefaultOverrideAndValidation(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{{"", 4}, {"1", 1}, {"25", 25}, {"64", 64}, {"128", 128}, {"256", 256}} {
		t.Run("value_"+tc.value, func(t *testing.T) {
			b := &backend{info: mediaget.Info{Title: "Synthetic"}}
			t.Setenv(DownloadEnv, t.TempDir())
			args := []string{"https://example.invalid", "--kind", "video", "--yes"}
			if tc.value != "" {
				args = append(args, "--concurrent-fragments", tc.value)
			}
			if err := app(t, b).Run(context.Background(), args, core.IO{}); err != nil {
				t.Fatal(err)
			}
			if b.last.Source.ConcurrentFragments != tc.want {
				t.Fatal(b.last.Source)
			}
		})
	}
	for _, value := range []string{"0", "257", "-1", "NaN", "4;exec"} {
		b := &backend{}
		t.Setenv(DownloadEnv, t.TempDir())
		err := app(t, b).Run(context.Background(), []string{"https://example.invalid", "--kind", "video", "--yes", "--concurrent-fragments", value}, core.IO{})
		if core.ExitCode(err) != 2 || b.calls != 0 || b.downloads != 0 {
			t.Fatalf("%q: %v calls=%d downloads=%d", value, err, b.calls, b.downloads)
		}
	}
}

func TestFragmentEnvironmentPrecedenceAndValidation(t *testing.T) {
	for _, tc := range []struct {
		name, env, flag string
		want            int
		invalid         bool
	}{
		{name: "environment", env: "25", want: 25},
		{name: "above_previous_limit", env: "128", want: 128},
		{name: "maximum", env: "256", want: 256},
		{name: "flag_overrides", env: "128", flag: "1", want: 1},
		{name: "flag_ignores_invalid_env", env: "invalid", flag: "25", want: 25},
		{name: "zero", env: "0", invalid: true},
		{name: "above_maximum", env: "257", invalid: true},
		{name: "negative", env: "-1", invalid: true},
		{name: "empty_is_invalid", env: "", invalid: true},
		{name: "non_numeric", env: "NaN", invalid: true},
		{name: "invalid_flag_beats_valid_env", env: "25", flag: "257", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(FragmentsEnv, tc.env)
			t.Setenv(DownloadEnv, t.TempDir())
			b := &backend{info: mediaget.Info{Title: "Synthetic"}}
			args := []string{"https://example.invalid", "--kind", "video", "--yes"}
			if tc.flag != "" {
				args = append(args, "--concurrent-fragments", tc.flag)
			}
			var log bytes.Buffer
			err := app(t, b).Run(context.Background(), args, core.IO{Err: &log})
			if tc.invalid {
				if core.ExitCode(err) != 2 || b.calls != 0 || b.downloads != 0 {
					t.Fatalf("err=%v calls=%d downloads=%d", err, b.calls, b.downloads)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if b.last.Source.ConcurrentFragments != tc.want || !strings.Contains(log.String(), "Fragmentos paralelos: "+strconv.Itoa(tc.want)) {
				t.Fatalf("request=%+v log=%s", b.last, log.String())
			}
		})
	}
}

func TestStaticEndpointsIgnoreInvalidFragmentEnvironment(t *testing.T) {
	t.Setenv(FragmentsEnv, "invalid")
	t.Setenv("HOME", "")
	b := &backend{}
	a := app(t, b)
	for _, args := range [][]string{{"--help"}, {"__cli", "contract"}, {"__cli", "schema"}, {"--version"}, {"completion", "generate", "zsh"}} {
		if err := a.Run(context.Background(), args, core.IO{}); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	if b.calls != 0 || b.downloads != 0 {
		t.Fatal("static endpoint reached backend")
	}
}
