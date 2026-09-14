package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestCapabilityPreflightRunsBeforeHandler(t *testing.T) {
	called := false
	app, err := Compile(App{
		ID: "caps", Name: "caps",
		Capabilities: []Capability{{ID: "atomic-replace", Availability: AvailabilityUnavailable}},
		Root:         Command{ID: "root", Name: "caps", Commands: []Command{{ID: "write", Name: "write", Capabilities: []CapabilityID{"atomic-replace"}, Handler: func(*Invocation) error { called = true; return nil }}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"write"}, IO{})
	if err == nil || ExitCode(err) != int(ExitUnavailable) {
		t.Fatalf("err=%v exit=%d", err, ExitCode(err))
	}
	if called {
		t.Fatal("handler ran before unavailable capability was rejected")
	}
}

func TestRequirementInheritedAndRunsBeforeHandler(t *testing.T) {
	called := false
	app, err := Compile(App{ID: "req", Name: "req", Root: Command{ID: "root", Name: "req", Requirements: []Requirement{{ID: "git", Check: func(context.Context) error { return errors.New("missing") }}}, Commands: []Command{{ID: "child", Name: "child", Handler: func(*Invocation) error { called = true; return nil }}}}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"child"}, IO{})
	if err == nil || ExitCode(err) != int(ExitUnavailable) {
		t.Fatalf("err=%v", err)
	}
	if called {
		t.Fatal("handler ran before inherited requirement")
	}
}

func TestMachineSchemaAndCompletionProtocol(t *testing.T) {
	app, err := Compile(App{ID: "machine", Name: "machine", Builtins: Builtins{Schema: true, Completion: true}, Root: Command{ID: "root", Name: "machine", Commands: []Command{{ID: "hello", Name: "hello", Summary: "say hello"}}}})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := app.Run(context.Background(), []string{"__cli", "schema"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	var schema Schema
	if err := json.Unmarshal(out.Bytes(), &schema); err != nil {
		t.Fatal(err)
	}
	if schema.AppID != "machine" || schema.Root.ID != "root" {
		t.Fatalf("%+v", schema)
	}
	out.Reset()
	req := `{"protocol":1,"argv":["machine","he"],"cursor_arg":1,"cursor_offset":2}`
	if err := app.Run(context.Background(), []string{"__cli", "complete"}, IO{In: strings.NewReader(req), Out: &out}); err != nil {
		t.Fatal(err)
	}
	var res CompletionResult
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 1 || res.Candidates[0].Value != "hello" {
		t.Fatalf("%+v", res)
	}
}

func TestMachineCompletionRejectsRequestsOverLimit(t *testing.T) {
	app, err := Compile(App{ID: "machine-limit", Name: "machine-limit", Builtins: Builtins{Completion: true}, Root: Command{ID: "root", Name: "machine-limit"}})
	if err != nil {
		t.Fatal(err)
	}
	valid := `{"protocol":1,"argv":["machine-limit"],"cursor_arg":0,"cursor_offset":0}`
	input := valid + strings.Repeat(" ", (1<<20)-len(valid)+1)
	err = app.Run(context.Background(), []string{"__cli", "complete"}, IO{In: strings.NewReader(input), Out: io.Discard})
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != CodeInvalidValue || !strings.Contains(d.Message, "1 MiB") {
		t.Fatalf("err=%#v", err)
	}
}

func TestShellCompletionTransportPreservesWhitespaceAndUnicode(t *testing.T) {
	app, err := Compile(App{
		ID: "transport", Name: "transport", Builtins: Builtins{Completion: true},
		Root: Command{
			ID: "root", Name: "transport",
			Args: []Arg{{
				ID: "value", Name: "value", Value: StringValue(),
				Completer: func(context.Context, CompleteContext) ([]CompletionCandidate, error) {
					return []CompletionCandidate{{Value: "olá mundo\tX", Description: "descrição com\ttab", Kind: CandidateValue}}, nil
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = app.Run(context.Background(), []string{"__cli", "complete-shell", "1", "bash", "1", "0", "--", "transport", ""}, IO{Out: &out})
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(out.String(), "\x00")
	joined := strings.Join(parts, "|")
	if !strings.Contains(joined, "olá mundo\tX") || !strings.Contains(joined, "descrição com\ttab") {
		t.Fatalf("transport lost candidate data: %q", out.Bytes())
	}
	if len(parts) < 3 || parts[0] != "cli-completion" || parts[1] != "1" {
		t.Fatalf("bad transport header: %#v", parts)
	}
}

func TestCompletionProtocolMismatchFailsClosed(t *testing.T) {
	app, err := Compile(App{ID: "x", Name: "x", Builtins: Builtins{Completion: true}, Root: Command{ID: "root", Name: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"__cli", "complete"}, IO{In: strings.NewReader(`{"protocol":99,"argv":["x"],"cursor_arg":0}`)})
	if err == nil || ExitCode(err) != int(ExitUsage) {
		t.Fatalf("err=%v exit=%d", err, ExitCode(err))
	}
}

func TestTextInteractionRefusesNonInteractive(t *testing.T) {
	p := TextInteraction{In: strings.NewReader("yes\n"), Out: &bytes.Buffer{}, Interactive: false}
	_, err := p.Confirm(context.Background(), Prompt{Message: "continue?"})
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != CodeNonInteractive {
		t.Fatalf("%v", err)
	}
}

func TestDoctorOutputIsDeterministic(t *testing.T) {
	app, err := Compile(App{ID: "d", Name: "d", Builtins: Builtins{Doctor: true}, Capabilities: []Capability{{ID: "z", Availability: AvailabilityAvailable}, {ID: "a", Availability: AvailabilityAvailable}}, DoctorChecks: []DoctorCheck{{ID: "m", Check: func(context.Context) DoctorResult { return DoctorResult{Status: DoctorHealthy} }}}, Root: Command{ID: "root", Name: "d"}})
	if err != nil {
		t.Fatal(err)
	}
	var a, b bytes.Buffer
	if err := app.Run(context.Background(), []string{"doctor", "--json"}, IO{Out: &a}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"doctor", "--json"}, IO{Out: &b}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("doctor output changed\n%s\n%s", a.Bytes(), b.Bytes())
	}
}

func TestContractDiffUsesStableIDs(t *testing.T) {
	oldC := Contract{Root: SchemaCommand{ID: "root", Name: "tool", Flags: []SchemaFlag{{ID: "f", Long: "old", Type: "string"}}}}
	newC := Contract{Root: SchemaCommand{ID: "root", Name: "tool", Flags: []SchemaFlag{{ID: "f", Long: "new", Type: "string"}, {ID: "g", Long: "extra", Type: "bool"}}}}
	changes := DiffContracts(oldC, newC)
	if len(changes) != 2 {
		t.Fatalf("%+v", changes)
	}
	seenBreaking, seenAdditive := false, false
	for _, c := range changes {
		seenBreaking = seenBreaking || c.Severity == ChangeBreaking
		seenAdditive = seenAdditive || c.Severity == ChangeAdditive
	}
	if !seenBreaking || !seenAdditive {
		t.Fatalf("%+v", changes)
	}
}

func TestContractDiffClassifiesRequiredArgAliasAndDefaultChanges(t *testing.T) {
	oldC := Contract{Root: SchemaCommand{ID: "root", Name: "tool", Aliases: []string{"t"}, Flags: []SchemaFlag{{ID: "f", Long: "format", Type: "string", Defaults: []string{"text"}, Aliases: []string{"fmt"}}}}}
	newC := Contract{Root: SchemaCommand{ID: "root", Name: "tool", Flags: []SchemaFlag{{ID: "f", Long: "format", Type: "string", Defaults: []string{"json"}}}, Args: []SchemaArg{{ID: "required", Name: "input", Type: "string", Required: true, Min: 1, Max: 1}}}}
	changes := DiffContracts(oldC, newC)
	want := map[string]ChangeSeverity{
		`removed command alias "t"`:                              ChangeBreaking,
		`removed flag alias "fmt"`:                               ChangeBreaking,
		`changed default value; semantic impact review required`: ChangeMetadata,
		`added argument`:                                         ChangeBreaking,
	}
	for msg, sev := range want {
		found := false
		for _, c := range changes {
			if c.Message == msg && c.Severity == sev {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing %s/%s in %+v", sev, msg, changes)
		}
	}
}

func TestCompilerRejectsInheritedGlobalFlagCollision(t *testing.T) {
	_, err := Compile(App{
		ID: "x", Name: "x",
		Root: Command{
			ID: "root", Name: "x",
			Flags:    []Flag{{ID: "global", Long: "json", Global: true, Action: FlagSwitch}},
			Commands: []Command{{ID: "child", Name: "child", Flags: []Flag{{ID: "local", Long: "json", Action: FlagSwitch}}}},
		},
	})
	if err == nil {
		t.Fatal("expected inherited flag collision")
	}
}

func TestCompilerRejectsInvalidDefaultAndConstraintKind(t *testing.T) {
	_, err := Compile(App{
		ID: "x", Name: "x",
		Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "n", Long: "number", Value: IntValue(), Default: []string{"nope"}}}},
	})
	if err == nil {
		t.Fatal("expected invalid default")
	}

	_, err = Compile(App{
		ID: "x", Name: "x",
		Root: Command{
			ID: "root", Name: "x",
			Flags:       []Flag{{ID: "a", Long: "a", Action: FlagSwitch}, {ID: "b", Long: "b", Action: FlagSwitch}},
			Constraints: []Constraint{{Kind: "bogus", IDs: []string{"a", "b"}}},
		},
	})
	if err == nil {
		t.Fatal("expected invalid constraint kind")
	}
}

func TestCompletionCursorOffsetUsesRunes(t *testing.T) {
	app, err := Compile(App{ID: "unicode", Name: "unicode", Builtins: Builtins{Completion: true}, Root: Command{ID: "root", Name: "unicode", Args: []Arg{{ID: "place", Name: "place", Value: EnumValue(Choice{Value: "área"}, Choice{Value: "árvore"})}}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"unicode", "ár"}, CursorArg: 1, CursorOffset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].Value != "área" || result.Candidates[1].Value != "árvore" {
		t.Fatalf("result=%+v", result)
	}
}

func TestDynamicCompletionIsBoundedAndNULSafe(t *testing.T) {
	app, err := Compile(App{
		ID: "bounded", Name: "bounded", Builtins: Builtins{Completion: true},
		Root: Command{
			ID: "root", Name: "bounded",
			Args: []Arg{{
				ID: "value", Name: "value", Value: StringValue(),
				Completer: func(context.Context, CompleteContext) ([]CompletionCandidate, error) {
					out := make([]CompletionCandidate, 0, MaxCompletionCandidates+20)
					out = append(out, CompletionCandidate{Value: "bad\x00value", Description: "must be filtered"})
					for i := 0; i < MaxCompletionCandidates+20; i++ {
						out = append(out, CompletionCandidate{Value: fmt.Sprintf("value-%03d", i), ID: fmt.Sprintf("id-%03d", i)})
					}
					return out, nil
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"bounded", ""}, CursorArg: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != MaxCompletionCandidates {
		t.Fatalf("candidate count=%d want=%d", len(result.Candidates), MaxCompletionCandidates)
	}
	for _, candidate := range result.Candidates {
		if strings.ContainsRune(candidate.Value, '\x00') {
			t.Fatalf("unsafe candidate survived: %#v", candidate)
		}
	}
}

func TestStaticFlagCompletionIsBounded(t *testing.T) {
	flags := make([]Flag, 0, MaxCompletionCandidates+40)
	for i := 0; i < MaxCompletionCandidates+40; i++ {
		flags = append(flags, Flag{ID: fmt.Sprintf("flag.%03d", i), Long: fmt.Sprintf("flag-%03d", i), Action: FlagSwitch})
	}
	app, err := Compile(App{ID: "bounded-static", Name: "bounded-static", Root: Command{ID: "root", Name: "bounded-static", Flags: flags}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"bounded-static", "--"}, CursorArg: 1, CursorOffset: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != MaxCompletionCandidates {
		t.Fatalf("candidate count=%d want=%d", len(result.Candidates), MaxCompletionCandidates)
	}
}

func TestCodecFuncsWithoutParserReturnsErrorInsteadOfPanicking(t *testing.T) {
	value := TypedValue(CodecFuncs[string]{Name: "custom"})
	app, err := Compile(App{ID: "codec", Name: "codec", Root: Command{ID: "root", Name: "codec", Args: []Arg{{ID: "value", Name: "value", Value: value}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"x"}, IO{}); err == nil || ExitCode(err) != int(ExitUsage) {
		t.Fatalf("err=%v", err)
	}
	if Sensitive(nil) != nil {
		t.Fatal("Sensitive(nil) must remain nil so Compile can reject a missing codec")
	}
}

func TestCompletionRejectsInvalidCursorCoordinates(t *testing.T) {
	app, err := Compile(App{ID: "cursor", Name: "cursor", Builtins: Builtins{Completion: true}, Root: Command{ID: "root", Name: "cursor"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []CompletionRequest{
		{Protocol: CompletionProtocol, Argv: []string{"cursor"}, CursorArg: -2},
		{Protocol: CompletionProtocol, Argv: []string{"cursor"}, CursorArg: 0, CursorOffset: -1},
		{Protocol: CompletionProtocol, Argv: []string{"cursor"}, CursorArg: 2},
		{Protocol: CompletionProtocol, Argv: []string{"cursor", "x"}, CursorArg: 1, CursorOffset: 2},
		{Protocol: CompletionProtocol, Argv: []string{"cursor"}, CursorArg: 1, CursorOffset: 1},
		{Protocol: CompletionProtocol, Argv: []string{"cursor"}, CursorArg: -1, CursorOffset: 1},
	} {
		if _, err := app.Complete(context.Background(), request); err == nil {
			t.Fatalf("expected invalid request to fail: %+v", request)
		}
	}
}

func TestCompletionCursorOffsetZeroMeansStartOfCurrentToken(t *testing.T) {
	app, err := Compile(App{ID: "cursor-zero", Name: "cursor-zero", Builtins: Builtins{Completion: true}, Root: Command{
		ID: "root", Name: "cursor-zero", Commands: []Command{{ID: "alpha", Name: "alpha"}, {ID: "beta", Name: "beta"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"cursor-zero", "beta"}, CursorArg: 1, CursorOffset: 0})
	if err != nil {
		t.Fatal(err)
	}
	foundAlpha := false
	for _, candidate := range result.Candidates {
		if candidate.Value == "alpha" {
			foundAlpha = true
			break
		}
	}
	if !foundAlpha {
		t.Fatalf("cursor at token start should use an empty prefix; alpha missing from %+v", result.Candidates)
	}
}

func TestTextInteractionSequentialPromptsDoNotReadAhead(t *testing.T) {
	in := strings.NewReader("profile\nyes\n")
	var out bytes.Buffer
	p := TextInteraction{In: in, Out: &out, Interactive: true}
	text, err := p.Text(context.Background(), Prompt{Message: "name"})
	if err != nil || text != "profile" {
		t.Fatalf("text=%q err=%v", text, err)
	}
	ok, err := p.Confirm(context.Background(), Prompt{Message: "confirm"})
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestResolutionPrecedenceAndProvenance(t *testing.T) {
	t.Setenv("TEST_FORMAT", "env")
	app, err := Compile(App{
		ID: "resolve", Name: "resolve",
		Root: Command{ID: "root", Name: "resolve", Flags: []Flag{{
			ID: "format", Long: "format", Value: StringValue(), Default: []string{"default"}, Providers: []ResolutionProvider{EnvProvider("TEST_FORMAT")},
		}}, Handler: func(inv *Invocation) error {
			values := inv.Values("format")
			if len(values) != 1 {
				t.Fatalf("values=%#v", values)
			}
			_, _ = fmt.Fprintf(inv.IO.Out, "%v|%s|%s", values[0].Value, values[0].Source, values[0].SourceName)
			return nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := app.Run(context.Background(), nil, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "env|env|TEST_FORMAT" {
		t.Fatalf("provider resolution=%q", got)
	}
	out.Reset()
	if err := app.Run(context.Background(), []string{"--format", "cli"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "cli|cli|" {
		t.Fatalf("CLI precedence=%q", got)
	}
	t.Setenv("TEST_FORMAT", "")
	if err := os.Unsetenv("TEST_FORMAT"); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := app.Run(context.Background(), nil, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "default|default|--format" {
		t.Fatalf("default provenance=%q", got)
	}
}

func TestRequiredFlagMayBeSatisfiedByProvider(t *testing.T) {
	app, err := Compile(App{ID: "required-provider", Name: "required-provider", Root: Command{
		ID: "root", Name: "required-provider",
		Flags: []Flag{{ID: "token", Long: "token", Value: StringValue(), Required: true, Providers: []ResolutionProvider{{Source: SourceUserConfig, Name: "auth.token", Resolve: func(context.Context) ([]string, bool, error) { return []string{"configured"}, true, nil }}}}},
		Handler: func(inv *Invocation) error {
			if got, ok := ValueAs[string](inv, "token"); !ok || got != "configured" {
				t.Fatalf("token=%q ok=%v", got, ok)
			}
			return nil
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), nil, IO{}); err != nil {
		t.Fatal(err)
	}
}

func TestFlagCountUsesIntegerCodecAndSchema(t *testing.T) {
	app, err := Compile(App{ID: "count", Name: "count", Root: Command{ID: "root", Name: "count", Flags: []Flag{{ID: "verbose", Long: "verbose", Short: 'v', Action: FlagCount}}, Handler: func(inv *Invocation) error {
		if got, ok := ValueAs[int64](inv, "verbose"); !ok || got != 2 {
			t.Fatalf("count=%v ok=%v", got, ok)
		}
		return nil
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if got := app.Schema().Root.Flags[0].Type; got != "int" {
		t.Fatalf("count schema type=%q", got)
	}
	if err := app.Run(context.Background(), []string{"-v", "-v"}, IO{}); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaIncludesResolutionSourceOrder(t *testing.T) {
	app, err := Compile(App{
		ID:   "sources",
		Name: "sources",
		Root: Command{
			ID:   "root",
			Name: "sources",
			Flags: []Flag{{
				ID:    "x",
				Long:  "x",
				Value: StringValue(),
				Providers: []ResolutionProvider{
					{Source: SourceProjectConfig, Name: "project.x", Resolve: func(context.Context) ([]string, bool, error) { return nil, false, nil }},
					{Source: SourceUserConfig, Name: "user.x", Resolve: func(context.Context) ([]string, bool, error) { return nil, false, nil }},
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sources := app.Schema().Root.Flags[0].Sources
	if len(sources) != 2 || sources[0].Name != "project.x" || sources[1].Name != "user.x" {
		t.Fatalf("sources=%#v", sources)
	}
}

func TestDiagnosticJSONAndSensitiveRedaction(t *testing.T) {
	err := &Diagnostic{Code: CodeInvalidValue, Kind: "value", Message: "token=super-secret", Hint: "secret hint", RelatedID: "token", Class: ExitUsage, Sensitive: true}
	var human bytes.Buffer
	RenderDiagnostic(&human, err)
	if strings.Contains(human.String(), "super-secret") || strings.Contains(human.String(), "secret hint") {
		t.Fatalf("sensitive human diagnostic leaked: %q", human.String())
	}
	var machine bytes.Buffer
	if e := RenderDiagnosticJSON(&machine, err); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(machine.String(), "super-secret") || strings.Contains(machine.String(), "secret hint") {
		t.Fatalf("sensitive JSON diagnostic leaked: %q", machine.String())
	}
	var got map[string]any
	if e := json.Unmarshal(machine.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if got["code"] != string(CodeInvalidValue) || got["exit_code"] != float64(ExitUsage) {
		t.Fatalf("unexpected JSON diagnostic: %#v", got)
	}
}

func TestSensitiveProviderFailuresNeverExposeProviderData(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider ResolutionProvider
	}{
		{
			name: "provider-error",
			provider: ResolutionProvider{Source: SourceProvider, Name: "secret-store", Resolve: func(context.Context) ([]string, bool, error) {
				return nil, false, errors.New("provider leaked super-secret")
			}},
		},
		{
			name: "invalid-provider-value",
			provider: ResolutionProvider{Source: SourceProvider, Name: "secret-store", Resolve: func(context.Context) ([]string, bool, error) {
				return []string{"super-secret"}, true, nil
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, err := Compile(App{ID: "sensitive-provider", Name: "sensitive-provider", Root: Command{ID: "root", Name: "sensitive-provider", Flags: []Flag{{ID: "token", Long: "token", Value: IntValue(), Sensitive: true, Providers: []ResolutionProvider{tc.provider}}}}})
			if err != nil {
				t.Fatal(err)
			}
			err = app.Run(context.Background(), nil, IO{})
			if err == nil {
				t.Fatal("expected resolution error")
			}
			if strings.Contains(err.Error(), "super-secret") || errors.Unwrap(err) != nil {
				t.Fatalf("sensitive provider data escaped through error: %#v", err)
			}
			var human, machine bytes.Buffer
			RenderDiagnostic(&human, err)
			if renderErr := RenderDiagnosticJSON(&machine, err); renderErr != nil {
				t.Fatal(renderErr)
			}
			if strings.Contains(human.String(), "super-secret") || strings.Contains(machine.String(), "super-secret") {
				t.Fatalf("sensitive provider data leaked: human=%q machine=%q", human.String(), machine.String())
			}
		})
	}
}

func TestSensitiveValuePredicateNeverExposesResolvedValue(t *testing.T) {
	app, err := Compile(App{ID: "sensitive-constraint", Name: "sensitive-constraint", Root: Command{
		ID: "root", Name: "sensitive-constraint",
		Flags: []Flag{{ID: "token", Long: "token", Value: StringValue(), Sensitive: true}},
		Constraints: []Constraint{{Kind: ValuePredicate, IDs: []string{"token"}, PredicateID: "token.valid", Validate: func(v ConstraintValues) error {
			token, _ := ConstraintValueAs[string](v, "token")
			return fmt.Errorf("rejected token %s", token)
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	err = app.Run(context.Background(), []string{"--token", "super-secret"}, IO{})
	if err == nil {
		t.Fatal("expected constraint failure")
	}
	if strings.Contains(err.Error(), "super-secret") || errors.Unwrap(err) != nil {
		t.Fatalf("sensitive constraint leaked through error: %#v", err)
	}
	var human bytes.Buffer
	RenderDiagnostic(&human, err)
	if strings.Contains(human.String(), "super-secret") {
		t.Fatalf("sensitive constraint leaked through renderer: %q", human.String())
	}
}

func TestSecretPromptNeverFallsBackToEchoedText(t *testing.T) {
	interaction := TextInteraction{In: strings.NewReader("secret\n"), Out: io.Discard, Interactive: true}
	if _, err := interaction.Secret(context.Background(), Prompt{Message: "secret"}); err == nil || ExitCode(err) != int(ExitUnavailable) {
		t.Fatalf("secret prompt err=%v exit=%d", err, ExitCode(err))
	}
	interaction.SecretReader = func(context.Context, Prompt) (string, error) { return "safe", nil }
	got, err := interaction.Secret(context.Background(), Prompt{Message: "secret"})
	if err != nil || got != "safe" {
		t.Fatalf("secret=%q err=%v", got, err)
	}
}

func TestOptionParsingPolicyIsExplicit(t *testing.T) {
	build := func(policy OptionPolicy) *CompiledApp {
		app, err := Compile(App{ID: "policy", Name: "policy", Root: Command{
			ID: "root", Name: "policy", OptionPolicy: policy,
			Args:  []Arg{{ID: "args", Name: "args", Value: StringValue(), Mode: ArgVariadic}},
			Flags: []Flag{{ID: "tag", Long: "tag", Value: StringValue()}},
			Handler: func(inv *Invocation) error {
				args, _ := ValuesAs[string](inv, "args")
				tag, _ := ValueAs[string](inv, "tag")
				_, _ = fmt.Fprintf(inv.IO.Out, "%q|%s", args, tag)
				return nil
			},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return app
	}
	var out bytes.Buffer
	if err := build(OptionsInterspersed).Run(context.Background(), []string{"a", "--tag", "x", "b"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[\"a\" \"b\"]|x" {
		t.Fatalf("interspersed=%q", got)
	}
	out.Reset()
	if err := build(OptionsBeforeArgs).Run(context.Background(), []string{"a", "--tag", "x"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); got != "[\"a\" \"--tag\" \"x\"]|" {
		t.Fatalf("before_args=%q", got)
	}
}

func TestOccurrenceAndValueConstraints(t *testing.T) {
	app, err := Compile(App{ID: "constraints", Name: "constraints", Root: Command{
		ID: "root", Name: "constraints",
		Flags: []Flag{{ID: "tag", Long: "tag", Value: StringValue(), Action: FlagAppend}, {ID: "mode", Long: "mode", Value: StringValue(), Default: []string{"safe"}}},
		Constraints: []Constraint{
			{Kind: MinOccurrences, IDs: []string{"tag"}, Min: 2},
			{Kind: MaxOccurrences, IDs: []string{"tag"}, Max: 3},
			{Kind: ValuePredicate, IDs: []string{"mode"}, PredicateID: "mode.allowed", Validate: func(v ConstraintValues) error {
				mode, _ := ConstraintValueAs[string](v, "mode")
				if mode != "safe" {
					return fmt.Errorf("mode must be safe")
				}
				return nil
			}},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"--tag", "a"}, IO{}); err == nil {
		t.Fatal("expected min occurrence failure")
	}
	if err := app.Run(context.Background(), []string{"--tag", "a", "--tag", "b"}, IO{}); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), []string{"--tag", "a", "--tag", "b", "--tag", "c", "--tag", "d"}, IO{}); err == nil {
		t.Fatal("expected max occurrence failure")
	}
	if err := app.Run(context.Background(), []string{"--tag", "a", "--tag", "b", "--mode", "unsafe"}, IO{}); err == nil {
		t.Fatal("expected value predicate failure")
	}
	schema := app.Schema()
	if len(schema.Root.Constraints) != 3 || schema.Root.Constraints[2].PredicateID != "mode.allowed" {
		t.Fatalf("constraints=%+v", schema.Root.Constraints)
	}
}

func TestCompileRejectsExpandedInvariantClasses(t *testing.T) {
	cases := []App{
		{ID: "bad id!", Name: "x", Root: Command{ID: "root", Name: "x"}},
		{ID: "x", Name: "x", Capabilities: []Capability{{ID: "cap", Availability: Availability(99)}}, Root: Command{ID: "root", Name: "x"}},
		{ID: "x", Name: "x", DoctorChecks: []DoctorCheck{{ID: "same", Check: func(context.Context) DoctorResult { return DoctorResult{} }}, {ID: "same", Check: func(context.Context) DoctorResult { return DoctorResult{} }}}, Root: Command{ID: "root", Name: "x"}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Outputs: []OutputFormat{OutputJSON, OutputJSON}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "a", Long: "a", Action: FlagSwitch}}, Constraints: []Constraint{{Kind: Conflicts, IDs: []string{"a", "root"}}}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Requirements: []Requirement{{ID: "git", Check: func(context.Context) error { return nil }}}, Commands: []Command{{ID: "child", Name: "child", Requirements: []Requirement{{ID: "git", Check: func(context.Context) error { return nil }}}}}}},
		{ID: "x", Name: "x", Middleware: []Middleware{nil}, Root: Command{ID: "root", Name: "x"}},
		{ID: "x", Name: "x", Builtins: Builtins{Help: true}, Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "help-flag", Long: "help", Action: FlagSwitch}}}},
		{ID: "x", Name: "x", Builtins: Builtins{Help: true}, Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "short-help", Long: "assist", Short: 'h', Action: FlagSwitch}}}},
		{ID: "x", Name: "x", Builtins: Builtins{Help: true}, Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "help-alias", Long: "assist", Aliases: []string{"help"}, Action: FlagSwitch}}}},
		{ID: "x", Name: "x", Builtins: Builtins{Version: true}, Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "version-flag", Long: "version", Action: FlagSwitch}}}},
		{ID: "x", Name: "x", Builtins: Builtins{Version: true}, Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "version-alias", Long: "release", Aliases: []string{"version"}, Action: FlagSwitch}}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Aliases: []string{"x"}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Aliases: []string{"alias", "alias"}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Args: []Arg{{ID: "a", Name: "same", Value: StringValue()}, {ID: "b", Name: "same", Value: StringValue()}}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Args: []Arg{{ID: "a", Name: "a", Value: StringValue(), Mode: ArgMode(99)}}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "a", Long: "a", Value: StringValue(), Action: FlagAction(99)}}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "a", Long: "a", Value: StringValue(), Action: FlagSwitch}}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "a", Long: "a", Value: StringValue(), Action: FlagCount}}}},
		{ID: "x", Name: "x", Root: Command{ID: "root", Name: "x", Flags: []Flag{{ID: "a", Long: "a", Value: StringValue(), Default: []string{"one", "two"}}}}},
	}
	for i, app := range cases {
		if _, err := Compile(app); err == nil {
			t.Fatalf("case %d: expected compile error", i)
		}
	}
}

func TestHelpShowsOptionalSubcommandWhenCommandAlsoHasHandler(t *testing.T) {
	app, err := Compile(App{ID: "help-handler", Name: "help-handler", Root: Command{ID: "root", Name: "help-handler", Commands: []Command{{
		ID: "parent", Name: "parent", Handler: func(*Invocation) error { return nil }, Commands: []Command{{ID: "child", Name: "child"}},
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	help := app.Help("parent")
	if !strings.Contains(help, "help-handler parent [command]") || strings.Contains(help, "help-handler parent <command>") {
		t.Fatalf("help misrepresents optional subcommand:\n%s", help)
	}
}

func TestDoctorIsPartOfCompiledGraph(t *testing.T) {
	app, err := Compile(App{ID: "doctor-graph", Name: "doctor-graph", Builtins: Builtins{Doctor: true}, Root: Command{ID: "root", Name: "doctor-graph"}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cmd := range app.Schema().Root.Commands {
		if cmd.ID == builtinDoctorCommandID {
			found = true
		}
	}
	if !found {
		t.Fatal("doctor missing from schema graph")
	}
}

func TestKnownUnavailableCommandsAreNotCompleted(t *testing.T) {
	app, err := Compile(App{ID: "availability", Name: "availability", Builtins: Builtins{Completion: true}, Capabilities: []Capability{{ID: "win-only", Availability: AvailabilityUnavailable}}, Root: Command{ID: "root", Name: "availability", Commands: []Command{{ID: "hidden-by-cap", Name: "special", Capabilities: []CapabilityID{"win-only"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"availability", "s"}, CursorArg: 1, CursorOffset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("unavailable command leaked into completion: %+v", result.Candidates)
	}
	fish, err := app.CompletionScript(ShellFish)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(fish, "special") {
		t.Fatalf("unavailable command leaked into static adapter:\n%s", fish)
	}
}

func TestCompletionPlannerOwnsAliasesBuiltinsAndPathHints(t *testing.T) {
	app, err := Compile(App{ID: "planner", Name: "planner", Builtins: Builtins{Help: true, Version: true, Completion: true}, Root: Command{
		ID: "root", Name: "planner", Commands: []Command{{
			ID: "serve", Name: "serve", Aliases: []string{"s"},
			Args: []Arg{{ID: "serve.path", Name: "path", Value: PathValue()}},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	alias, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"planner", "s"}, CursorArg: 1, CursorOffset: 1})
	if err != nil {
		t.Fatal(err)
	}
	foundAlias := false
	for _, c := range alias.Candidates {
		if c.Value == "s" && c.Kind == CandidateCommand {
			foundAlias = true
		}
	}
	if !foundAlias {
		t.Fatalf("command alias missing from runtime planner: %+v", alias.Candidates)
	}
	flags, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"planner", "--"}, CursorArg: 1, CursorOffset: 2})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range flags.Candidates {
		if c.Value == "--help" || c.Value == "--version" {
			t.Fatalf("synthetic execution-only builtin leaked into completion candidates: %+v", flags.Candidates)
		}
	}
	paths, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"planner", "serve", ""}, CursorArg: 2, CursorOffset: 0})
	if err != nil {
		t.Fatal(err)
	}
	if !paths.Directive.Files || !paths.Directive.Directories {
		t.Fatalf("HintPath must request files and directories: %+v", paths.Directive)
	}
}

func TestDynamicCompletionReceivesTypedPriorValuesWithoutSensitiveData(t *testing.T) {
	var gotEnv string
	var leakedSensitive bool
	app, err := Compile(App{ID: "contextual", Name: "contextual", Builtins: Builtins{Completion: true}, Root: Command{
		ID: "root", Name: "contextual", Commands: []Command{{
			ID: "deploy", Name: "deploy",
			Args: []Arg{
				{ID: "deploy.environment", Name: "environment", Value: EnumValue(Choice{Value: "dev"}, Choice{Value: "prod"}), Required: true},
				{ID: "deploy.target", Name: "target", Value: StringValue(), Completer: func(_ context.Context, cc CompleteContext) ([]CompletionCandidate, error) {
					gotEnv, _ = CompletionValueAs[string](cc, "deploy.environment")
					_, leakedSensitive = CompletionValueAs[string](cc, "deploy.token")
					return []CompletionCandidate{{Value: gotEnv + "-target", Kind: CandidateValue}}, nil
				}},
			},
			Flags: []Flag{{ID: "deploy.token", Long: "token", Value: StringValue(), Sensitive: true}},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"contextual", "deploy", "prod", "--token", "secret", "p"}, CursorArg: 5, CursorOffset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if gotEnv != "prod" || leakedSensitive {
		t.Fatalf("context env=%q leakedSensitive=%v", gotEnv, leakedSensitive)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Value != "prod-target" {
		t.Fatalf("result=%+v", result)
	}
}

func TestDynamicCompletionHonorsCancellationBeforeProvider(t *testing.T) {
	called := false
	app, err := Compile(App{ID: "cancel-complete", Name: "cancel-complete", Builtins: Builtins{Completion: true}, Root: Command{
		ID: "root", Name: "cancel-complete",
		Args: []Arg{{ID: "target", Name: "target", Value: StringValue(), Completer: func(context.Context, CompleteContext) ([]CompletionCandidate, error) {
			called = true
			return []CompletionCandidate{{Value: "should-not-run"}}, nil
		}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = app.Complete(ctx, CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"cancel-complete", ""}, CursorArg: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("completion err=%v, want context.Canceled", err)
	}
	if called {
		t.Fatal("dynamic provider ran after context was already canceled")
	}
}

func TestContractDiffClassifiesProtocolAndSchemaVersions(t *testing.T) {
	oldC := Contract{Version: 1, SchemaVersion: 1, CompletionProtocol: 1, AppID: "app", Name: "tool", Root: SchemaCommand{ID: "root", Name: "tool"}}
	newC := oldC
	newC.Version = 2
	newC.SchemaVersion = 2
	newC.CompletionProtocol = 2
	newC.AppID = "app-v2"
	changes := DiffContracts(oldC, newC)
	for _, id := range []string{"$contract", "$schema", "$completion", "$app"} {
		found := false
		for _, change := range changes {
			if change.ID == id && change.Severity == ChangeBreaking {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing breaking classification for %s in %+v", id, changes)
		}
	}
}

func TestInvocationExposesCapabilitiesAndMiddlewareOrder(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(inv *Invocation) error {
				order = append(order, name+":before")
				err := next(inv)
				order = append(order, name+":after")
				return err
			}
		}
	}
	app, err := Compile(App{
		ID: "middleware", Name: "middleware",
		Capabilities: []Capability{{ID: "optional", Summary: "optional feature", Availability: AvailabilityAvailable}},
		Middleware:   []Middleware{mw("outer"), mw("inner")},
		Root: Command{ID: "root", Name: "middleware", Handler: func(inv *Invocation) error {
			order = append(order, "handler")
			cap, ok := inv.Capability("optional")
			if !ok || cap.Availability != AvailabilityAvailable || cap.Summary != "optional feature" {
				t.Fatalf("capability=%+v ok=%v", cap, ok)
			}
			return nil
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(context.Background(), nil, IO{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"outer:before", "inner:before", "handler", "inner:after", "outer:after"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("middleware order=%v want=%v", order, want)
	}
}
