package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func fixture(t *testing.T) *CompiledApp {
	t.Helper()
	app := App{ID: "test", Name: "tool", Product: ProductMetadata{Version: "1.2.3", SuiteVersion: "v9.8.7", Stability: "beta"}, Builtins: Builtins{Help: true, Version: true}, Root: Command{ID: "root", Name: "tool", Flags: []Flag{{ID: "verbose", Long: "verbose", Short: 'v', Action: FlagSwitch, Global: true, Summary: "verbose output"}}, Commands: []Command{{ID: "copy", Name: "copy", Summary: "copy a thing", Args: []Arg{{ID: "src", Name: "source", Value: PathValue(), Required: true}, {ID: "rest", Name: "args", Value: StringValue(), Mode: ArgOpaque}}, Flags: []Flag{{ID: "format", Long: "format", Short: 'f', Value: EnumValue(Choice{Value: "json"}, Choice{Value: "text"})}}, Constraints: []Constraint{{Kind: Conflicts, IDs: []string{"verbose", "format"}}}, Handler: func(inv *Invocation) error {
		src, _ := ValueAs[string](inv, "src")
		rest, _ := ValuesAs[string](inv, "rest")
		_, err := inv.IO.Out.Write([]byte(src + ":" + strings.Join(rest, ",")))
		return err
	}}}}}
	c, err := Compile(app)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCompileRejectsDuplicateStableID(t *testing.T) {
	_, err := Compile(App{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Args: []Arg{{ID: "dup", Name: "a", Value: StringValue()}}, Flags: []Flag{{ID: "dup", Long: "b", Value: StringValue()}}}})
	if err == nil {
		t.Fatal("expected duplicate ID error")
	}
}
func TestCompileRejectsVariadicBeforeAnotherArg(t *testing.T) {
	_, err := Compile(App{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Args: []Arg{{ID: "a", Name: "a", Value: StringValue(), Mode: ArgVariadic}, {ID: "b", Name: "b", Value: StringValue()}}}})
	if err == nil {
		t.Fatal("expected variadic ordering error")
	}
}
func TestRunTypedOpaqueAndGlobalFlag(t *testing.T) {
	c := fixture(t)
	var out bytes.Buffer
	err := c.Run(context.Background(), []string{"-v", "copy", "src", "--child-flag", "x"}, IO{Out: &out})
	if err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "src:--child-flag,x" {
		t.Fatalf("got %q", got)
	}
}
func TestConstraintDiagnosticExit2(t *testing.T) {
	c := fixture(t)
	err := c.Run(context.Background(), []string{"-v", "copy", "--format", "json", "src"}, IO{})
	if err == nil {
		t.Fatal("expected error")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("exit=%d", ExitCode(err))
	}
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != CodeConstraint {
		t.Fatalf("diag=%v", err)
	}
}
func TestUnknownCommandSuggestion(t *testing.T) {
	c := fixture(t)
	err := c.Run(context.Background(), []string{"cpy"}, IO{})
	var d *Diagnostic
	if !errors.As(err, &d) {
		t.Fatalf("%v", err)
	}
	if d.Code != CodeUnknownCommand || !strings.Contains(d.Hint, "copy") {
		t.Fatalf("diag=%+v", d)
	}
}
func TestHelpAndVersionAreStatic(t *testing.T) {
	c := fixture(t)
	if !strings.Contains(c.Help(), "copy") || !strings.Contains(c.Help(), "--verbose") {
		t.Fatal(c.Help())
	}
	var out bytes.Buffer
	if err := c.Run(context.Background(), []string{"--version"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "tool 1.2.3\n" {
		t.Fatal(out.String())
	}
}

func TestVersionCommandJSON(t *testing.T) {
	c := fixture(t)
	var out bytes.Buffer
	if err := c.Run(context.Background(), []string{"version", "--json"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	var info VersionInfo
	if err := json.Unmarshal(out.Bytes(), &info); err != nil {
		t.Fatal(err)
	}
	if info.Tool != "tool" || info.Version != "1.2.3" || info.SuiteVersion != "v9.8.7" || info.Stability != "beta" {
		t.Fatalf("info=%+v", info)
	}
	if info.SchemaVersion != SchemaVersion || info.CompletionProtocol != CompletionProtocol || info.GoVersion == "" || info.OS == "" || info.Arch == "" {
		t.Fatalf("missing build metadata: %+v", info)
	}
}

func TestNestedHelpResolvesActiveCommandAndRespectsDoubleDash(t *testing.T) {
	c := fixture(t)
	var out bytes.Buffer
	if err := c.Run(context.Background(), []string{"copy", "--help"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Usage:\n  tool copy") {
		t.Fatalf("nested help did not target copy:\n%s", out.String())
	}
	out.Reset()
	if err := c.Run(context.Background(), []string{"copy", "src", "--help"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "src:--help" {
		t.Fatalf("help inside opaque argv was intercepted: %q", got)
	}
	out.Reset()
	if err := c.Run(context.Background(), []string{"copy", "src", "--", "--help"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "src:--,--help" {
		t.Fatalf("help after -- was intercepted: %q", got)
	}
}

func TestCompletionEnumAndCommands(t *testing.T) {
	c := fixture(t)
	r, err := c.Complete(context.Background(), CompletionRequest{Protocol: 1, Argv: []string{"tool", "co"}, CursorArg: 1, CursorOffset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) == 0 || r.Candidates[0].Value != "copy" {
		t.Fatalf("%+v", r)
	}
	r, err = c.Complete(context.Background(), CompletionRequest{Protocol: 1, Argv: []string{"tool", "copy", "--format="}, CursorArg: 2, CursorOffset: 9})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) != 2 || r.Candidates[0].Value != "--format=json" || !r.Directive.NoSpace {
		t.Fatalf("%+v", r)
	}
}
func TestCompletionIncludesCommandAndFlagAliases(t *testing.T) {
	c, err := Compile(App{ID: "aliases", Name: "aliases", Root: Command{ID: "root", Name: "aliases", Commands: []Command{{ID: "remove", Name: "remove", Aliases: []string{"rm"}, Flags: []Flag{{ID: "force", Long: "force", Aliases: []string{"overwrite"}, Action: FlagSwitch}}}}}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.Complete(context.Background(), CompletionRequest{Protocol: 1, Argv: []string{"aliases", "r"}, CursorArg: 1, CursorOffset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) != 2 {
		t.Fatalf("command aliases: %+v", r)
	}
	r, err = c.Complete(context.Background(), CompletionRequest{Protocol: 1, Argv: []string{"aliases", "rm", "--o"}, CursorArg: 2, CursorOffset: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) != 1 || r.Candidates[0].Value != "--overwrite" {
		t.Fatalf("flag aliases: %+v", r)
	}
}

func TestSchemaDeterministicAndSensitiveChoicesRedacted(t *testing.T) {
	secret := Sensitive(EnumValue(Choice{Value: "token"}))
	c, err := Compile(App{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "secret", Long: "secret", Value: secret}}}})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := c.SchemaJSON()
	b, _ := c.SchemaJSON()
	if !bytes.Equal(a, b) {
		t.Fatal("schema is nondeterministic")
	}
	if bytes.Contains(a, []byte("token")) {
		t.Fatal("sensitive choice leaked")
	}
}

func TestCompileRejectsUnsafeShellTokens(t *testing.T) {
	cases := []struct {
		name string
		app  App
	}{
		{name: "app whitespace", app: App{ID: "x", Name: "bad name", Root: Command{ID: "root", Name: "bad name"}}},
		{name: "command control", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Commands: []Command{
			{ID: "bad", Name: "bad\nname"},
		}}}},
		{name: "command path", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Commands: []Command{
			{ID: "bad", Name: "../bad"},
		}}}},
		{name: "command shell metachar", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Commands: []Command{
			{ID: "bad", Name: "bad;echo"},
		}}}},
		{name: "alias shell metachar", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Commands: []Command{
			{ID: "bad", Name: "good", Aliases: []string{"bad$alias"}},
		}}}},
		{name: "alias leading dash", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Commands: []Command{
			{ID: "bad", Name: "good", Aliases: []string{"--oops"}},
		}}}},
		{name: "flag equals", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Flags: []Flag{
			{ID: "bad", Long: "x=y", Value: StringValue()},
		}}}},
		{name: "flag alias whitespace", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Flags: []Flag{
			{ID: "bad", Long: "good", Aliases: []string{"bad alias"}, Value: StringValue()},
		}}}},
		{name: "short whitespace", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Flags: []Flag{
			{ID: "bad", Long: "good", Short: ' ', Value: StringValue()},
		}}}},
		{name: "short shell metachar", app: App{ID: "x", Name: "tool", Root: Command{ID: "root", Name: "tool", Flags: []Flag{
			{ID: "bad", Long: "good", Short: ';', Value: StringValue()},
		}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compile(tc.app); err == nil {
				t.Fatal("expected unsafe token to be rejected")
			}
		})
	}
}

func TestSensitiveFieldSuppressesCompletionWithoutSensitiveCodec(t *testing.T) {
	called := false
	app, err := Compile(App{ID: "secret", Name: "secret", Builtins: Builtins{Completion: true}, Root: Command{
		ID: "root", Name: "secret",
		Flags: []Flag{{ID: "token", Long: "token", Value: EnumValue(Choice{Value: "should-not-leak"}), Sensitive: true, Completer: func(context.Context, CompleteContext) ([]CompletionCandidate, error) {
			called = true
			return []CompletionCandidate{{Value: "dynamic-secret"}}, nil
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"--token", ""}, CursorArg: 1})
	if err != nil {
		t.Fatal(err)
	}
	if called || len(got.Candidates) != 0 {
		t.Fatalf("sensitive completion leaked: called=%v candidates=%#v", called, got.Candidates)
	}
}

func TestSensitiveInvalidValueIsRedacted(t *testing.T) {
	app, err := Compile(App{ID: "secret", Name: "secret", Root: Command{ID: "root", Name: "secret", Flags: []Flag{{ID: "token", Long: "token", Value: IntValue(), Sensitive: true}}}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"--token", "super-secret"}, IO{})
	if err == nil {
		t.Fatal("expected invalid value")
	}
	var out bytes.Buffer
	RenderDiagnostic(&out, err)
	if strings.Contains(out.String(), "super-secret") {
		t.Fatalf("sensitive parse value leaked: %q", out.String())
	}
}

func TestCompletionRespectsEndOfOptionsAndBeforeArgsPolicy(t *testing.T) {
	build := func(policy OptionPolicy) *CompiledApp {
		app, err := Compile(App{ID: "complete-policy", Name: "complete-policy", Builtins: Builtins{Completion: true}, Root: Command{
			ID: "root", Name: "complete-policy", OptionPolicy: policy,
			Args:  []Arg{{ID: "args", Name: "args", Value: StringValue(), Mode: ArgVariadic}},
			Flags: []Flag{{ID: "flag", Long: "flag", Action: FlagSwitch}},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return app
	}
	for _, tc := range []struct {
		name   string
		app    *CompiledApp
		argv   []string
		cursor int
	}{
		{name: "double-dash", app: build(OptionsInterspersed), argv: []string{"--", "--f"}, cursor: 1},
		{name: "before-args", app: build(OptionsBeforeArgs), argv: []string{"arg", "--f"}, cursor: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: tc.argv, CursorArg: tc.cursor, CursorOffset: 3})
			if err != nil {
				t.Fatal(err)
			}
			for _, candidate := range got.Candidates {
				if candidate.Kind == CandidateFlag {
					t.Fatalf("flag suggested after option parsing ended: %#v", got.Candidates)
				}
			}
		})
	}
}

func TestNegativeNumericPositionalsAreNotMisparsedAsFlags(t *testing.T) {
	app, err := Compile(App{ID: "negative", Name: "negative", Root: Command{ID: "root", Name: "negative", Args: []Arg{{ID: "n", Name: "n", Value: FloatValue(), Required: true}}, Handler: func(inv *Invocation) error {
		v, ok := ValueAs[float64](inv, "n")
		if !ok || v != -0.5 {
			t.Fatalf("v=%v ok=%v", v, ok)
		}
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"-0.5"}, IO{}); err != nil {
		t.Fatal(err)
	}
}

func TestShortClustersAreExplicitlyUnsupported(t *testing.T) {
	app, err := Compile(App{ID: "clusters", Name: "clusters", Root: Command{ID: "root", Name: "clusters", Flags: []Flag{{ID: "a", Long: "all", Short: 'a', Action: FlagSwitch}, {ID: "b", Long: "brief", Short: 'b', Action: FlagSwitch}}}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"-ab"}, IO{})
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != CodeUnknownFlag {
		t.Fatalf("err=%#v", err)
	}
}

func TestCompletionMatchesNegativePositionalGrammar(t *testing.T) {
	app, err := Compile(App{ID: "negative-complete", Name: "negative-complete", Builtins: Builtins{Completion: true}, Root: Command{ID: "root", Name: "negative-complete", Args: []Arg{{ID: "n", Name: "n", Value: FloatValue()}}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"negative-complete", "-0"}, CursorArg: 1, CursorOffset: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got.Candidates {
		if c.Kind == CandidateFlag {
			t.Fatalf("numeric positional prefix treated as flag: %+v", got)
		}
	}
}
