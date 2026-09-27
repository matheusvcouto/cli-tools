package profilecli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/matheusvcouto/cli-tools/v2/cli"
	"github.com/matheusvcouto/cli-tools/v2/internal/aiprofile"
)

type fakeRunner struct {
	binary string
	args   []string
	calls  int
}

func (f *fakeRunner) Replace(_ context.Context, binary string, args []string, _ []string, _ aiprofile.ProcessIO) error {
	f.calls++
	f.binary = binary
	f.args = append([]string(nil), args...)
	return nil
}

func testService(t *testing.T) (*aiprofile.Service, *fakeRunner) {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	return &aiprofile.Service{
		Store:   aiprofile.Store{Root: filepath.Join(base, "profiles")},
		Runner:  runner,
		HomeDir: home,
		Now:     func() time.Time { return time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC) },
		Env:     func() []string { return []string{"PATH=/synthetic/bin"} },
	}, runner
}

func TestStaticPathsDoNotInitializeService(t *testing.T) {
	calls := 0
	lazy := core.NewLazy(func() (*aiprofile.Service, error) {
		calls++
		return &aiprofile.Service{Store: aiprofile.Store{Root: filepath.Join(t.TempDir(), "profiles")}, Runner: &fakeRunner{}}, nil
	})
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := app.Run(context.Background(), []string{"--help"}, core.IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("help initialized service %d times", calls)
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"--version"}, core.IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("version initialized service %d times", calls)
	}
	r, err := app.Complete(context.Background(), core.CompletionRequest{Protocol: 1, Argv: []string{"ai-profile", "cl"}, CursorArg: 1, CursorOffset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) == 0 || r.Candidates[0].Value != "claude" {
		t.Fatalf("%+v", r)
	}
	r, err = app.Complete(context.Background(), core.CompletionRequest{Protocol: 1, Argv: []string{"ai-profile", "gr"}, CursorArg: 1, CursorOffset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) == 0 || r.Candidates[0].Value != "grok" {
		t.Fatalf("missing static Grok completion: %+v", r)
	}
	if calls != 0 {
		t.Fatalf("static completion initialized service %d times", calls)
	}
}

func TestDynamicProfileCompletionIsLazyAndSorted(t *testing.T) {
	service, _ := testService(t)
	for _, name := range []string{"z", "a"} {
		if _, err := service.Create("claude", name); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	lazy := core.NewLazy(func() (*aiprofile.Service, error) { calls++; return service, nil })
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := app.Complete(context.Background(), core.CompletionRequest{Protocol: 1, Argv: []string{"ai-profile", "claude", "delete", ""}, CursorArg: 3, CursorOffset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("dynamic completion init calls=%d", calls)
	}
	if len(r.Candidates) != 2 || r.Candidates[0].Value != "a" || r.Candidates[1].Value != "z" {
		t.Fatalf("%+v", r)
	}
}

func TestACPUsesOpaqueTrailingArgvWithoutWrapperOutput(t *testing.T) {
	service, runner := testService(t)
	if _, err := service.Create("claude", "p"); err != nil {
		t.Fatal(err)
	}
	lazy := core.NewLazy(func() (*aiprofile.Service, error) { return service, nil })
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := app.Run(context.Background(), []string{"claude", "acp", "p", "--flag"}, core.IO{Out: &out, Err: &errOut}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("wrapper output stdout=%q stderr=%q", out.String(), errOut.String())
	}
	if runner.binary != "claude-agent-acp" || strings.Join(runner.args, "|") != "--flag" {
		t.Fatalf("bad ACP call: %s %#v", runner.binary, runner.args)
	}
}

func TestGrokACPPlacesAgentOptionsBeforeStdio(t *testing.T) {
	service, runner := testService(t)
	if _, err := service.Create("grok", "p"); err != nil {
		t.Fatal(err)
	}
	lazy := core.NewLazy(func() (*aiprofile.Service, error) { return service, nil })
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if err := app.Run(context.Background(), []string{"grok", "acp", "p", "--model", "grok-4.7"}, core.IO{Out: &out, Err: &errOut}); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("wrapper output stdout=%q stderr=%q", out.String(), errOut.String())
	}
	if runner.binary != "grok" || strings.Join(runner.args, "|") != "agent|--model|grok-4.7|stdio" {
		t.Fatalf("bad Grok ACP call: %s %#v", runner.binary, runner.args)
	}
}

func TestListJSONStableAndSorted(t *testing.T) {
	service, _ := testService(t)
	for _, name := range []string{"z", "a"} {
		if _, err := service.Create("claude", name); err != nil {
			t.Fatal(err)
		}
	}
	lazy := core.NewLazy(func() (*aiprofile.Service, error) { return service, nil })
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := app.Run(context.Background(), []string{"claude", "list", "--json"}, core.IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"profile": "a"`) {
		t.Fatalf("missing JSON: %s", out.String())
	}
	if strings.Index(out.String(), `"profile": "a"`) > strings.Index(out.String(), `"profile": "z"`) {
		t.Fatalf("not sorted: %s", out.String())
	}
}

func TestContractLockIsCurrentWithoutInitializingService(t *testing.T) {
	calls := 0
	lazy := core.NewLazy(func() (*aiprofile.Service, error) {
		calls++
		return nil, errors.New("must not initialize")
	})
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "0.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.ContractJSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../cmd/ai-profile/cli.contract.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("ai-profile cli.contract.json is stale; regenerate it from __cli contract")
	}
	if calls != 0 {
		t.Fatalf("contract generation initialized service %d times", calls)
	}
}

func TestDeleteUsesCoreInteractionAndRequiresTwoConfirmations(t *testing.T) {
	service, _ := testService(t)
	if _, err := service.Create("claude", "profile"); err != nil {
		t.Fatal(err)
	}
	lazy := core.NewLazy(func() (*aiprofile.Service, error) { return service, nil })
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	io := core.IO{
		In:       strings.NewReader("profile\nyes\n"),
		Out:      &out,
		Err:      &errOut,
		Terminal: core.Terminal{StdinTTY: true},
	}
	if err := app.Run(context.Background(), []string{"claude", "delete", "profile"}, io); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.Profile("claude", "profile"); err == nil {
		t.Fatal("profile still exists after confirmed delete")
	}
	if !strings.Contains(errOut.String(), `Type "profile" to confirm`) || !strings.Contains(errOut.String(), "Delete permanently?") {
		t.Fatalf("unexpected confirmation output: %q", errOut.String())
	}
}

type triggerOnFirstRead struct {
	reader  *strings.Reader
	trigger func() error
	called  bool
}

func (r *triggerOnFirstRead) Read(p []byte) (int, error) {
	if !r.called {
		r.called = true
		if err := r.trigger(); err != nil {
			return 0, err
		}
	}
	return r.reader.Read(p)
}

// Reproduces an alias being reassigned after the displayed path is chosen
// but before a human finishes answering deletion prompts. Neither directory
// may be removed, regardless of prompt input.
func TestInteractiveDeleteRefusesProfileReplacedDuringConfirmation(t *testing.T) {
	service, _ := testService(t)
	original, err := service.Create("grok", "profile")
	if err != nil {
		t.Fatal(err)
	}
	var replacement aiprofile.Profile
	input := &triggerOnFirstRead{
		reader: strings.NewReader("profile\nyes\n"),
		trigger: func() error {
			if _, err := service.Rename("grok", "profile", "preserved"); err != nil {
				return err
			}
			var err error
			replacement, err = service.Create("grok", "profile")
			return err
		},
	}
	lazy := core.NewLazy(func() (*aiprofile.Service, error) { return service, nil })
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	err = app.Run(context.Background(), []string{"grok", "delete", "profile"}, core.IO{
		In: input, Out: &out, Err: &errOut, Terminal: core.Terminal{StdinTTY: true},
	})
	if err == nil || !strings.Contains(err.Error(), "changed after deletion confirmation") {
		t.Fatalf("interactive delete accepted a replaced alias: %v", err)
	}
	if !input.called || replacement.Dir == "" || original.Dir == replacement.Dir {
		t.Fatalf("test did not replace the alias: original=%s replacement=%s", original.Dir, replacement.Dir)
	}
	for _, profile := range []aiprofile.Profile{original, replacement} {
		if _, err := os.Stat(filepath.Join(profile.Dir, "config.toml")); err != nil {
			t.Fatalf("unsafe delete altered %s: %v", profile.Dir, err)
		}
	}
}

func TestDeleteRefusesNonInteractiveInputBeforeMutation(t *testing.T) {
	service, _ := testService(t)
	if _, err := service.Create("claude", "profile"); err != nil {
		t.Fatal(err)
	}
	lazy := core.NewLazy(func() (*aiprofile.Service, error) { return service, nil })
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"claude", "delete", "profile"}, core.IO{In: strings.NewReader("profile\nyes\n")})
	var d *core.Diagnostic
	if !errors.As(err, &d) || d.Code != core.CodeNonInteractive {
		t.Fatalf("err=%#v", err)
	}
	if _, _, err := service.Profile("claude", "profile"); err != nil {
		t.Fatalf("non-interactive delete mutated profile: %v", err)
	}
}

func TestToolCommandSurfaceFollowsToolCapabilities(t *testing.T) {
	lazy := core.NewLazy(func() (*aiprofile.Service, error) {
		return nil, errors.New("must not initialize")
	})

	withoutOptional := toolCommand(aiprofile.ToolSpec{Name: "plain", Binary: "plain", ConfigEnv: "PLAIN_HOME"}, lazy, aiprofile.ProcessIO{})
	if hasCommand(withoutOptional.Commands, "acp") {
		t.Fatalf("unsupported commands leaked into plain tool: %+v", commandNames(withoutOptional.Commands))
	}

	withOptional := toolCommand(aiprofile.ToolSpec{
		Name:      "full",
		Binary:    "full",
		ConfigEnv: "FULL_HOME",
		ACP:       &aiprofile.ACPTool{Binary: "full-acp"},
	}, lazy, aiprofile.ProcessIO{})
	if !hasCommand(withOptional.Commands, "acp") {
		t.Fatalf("capability command missing from full tool: %+v", commandNames(withOptional.Commands))
	}
}

func TestApplyStatuslineIsRetiredFromActiveCLI(t *testing.T) {
	lazy := core.NewLazy(func() (*aiprofile.Service, error) {
		return nil, errors.New("must not initialize")
	})
	app, err := New(lazy, aiprofile.ProcessIO{}, core.ProductMetadata{Version: "test"})
	if err != nil {
		t.Fatal(err)
	}

	for _, tool := range []string{"claude", "codex", "grok"} {
		var help bytes.Buffer
		if err := app.Run(context.Background(), []string{tool, "--help"}, core.IO{Out: &help}); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(help.String(), "apply-statusline") {
			t.Fatalf("%s still advertises retired apply-statusline command:\n%s", tool, help.String())
		}

		err := app.Run(context.Background(), []string{tool, "apply-statusline", "profile"}, core.IO{})
		var diagnostic *core.Diagnostic
		if !errors.As(err, &diagnostic) || diagnostic.Code != core.CodeUnknownCommand {
			t.Fatalf("%s retired command unexpectedly accepted: %#v", tool, err)
		}
	}
}

func hasCommand(commands []core.Command, name string) bool {
	for _, command := range commands {
		if command.Name == name {
			return true
		}
	}
	return false
}

func commandNames(commands []core.Command) []string {
	names := make([]string, 0, len(commands))
	for _, command := range commands {
		names = append(names, command.Name)
	}
	return names
}
