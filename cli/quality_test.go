package cli

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

func TestCompileRejectsDeprecationAndModuleConflicts(t *testing.T) {
	if _, err := Compile(App{
		ID: "deprecation", Name: "deprecation",
		Root: Command{ID: "root", Name: "deprecation", Commands: []Command{{
			ID: "old", Name: "old", Deprecated: &Deprecation{ReplacementID: "missing"},
		}}},
	}); err == nil {
		t.Fatal("expected unknown deprecation replacement to fail compilation")
	}
	if _, err := Compile(App{
		ID: "deprecation-self", Name: "deprecation-self",
		Root: Command{ID: "root", Name: "deprecation-self", Commands: []Command{{
			ID: "old", Name: "old", Deprecated: &Deprecation{ReplacementID: "old"},
		}}},
	}); err == nil {
		t.Fatal("expected self deprecation replacement to fail compilation")
	}

	add := func(id string) Module {
		return ModuleFunc(func(a *App) error {
			a.Root.Commands = append(a.Root.Commands, Command{ID: id, Name: "same"})
			return nil
		})
	}
	if _, err := Compile(App{
		ID: "modules", Name: "modules", Root: Command{ID: "root", Name: "modules"},
		Modules: []Module{add("module.one"), add("module.two")},
	}); err == nil {
		t.Fatal("expected conflicting module command names to fail compilation")
	}
}

func TestUnknownShellAdapterFailsClosed(t *testing.T) {
	app := shellFixture(t)
	if _, err := app.CompletionScript(Shell("tcsh")); err == nil {
		t.Fatal("expected unsupported shell adapter to fail")
	}
	if _, err := ShellCapabilities(Shell("tcsh")); err == nil {
		t.Fatal("expected unsupported shell capability lookup to fail")
	}
}

func TestStaticCompletionPrunesInheritedUnavailableCapability(t *testing.T) {
	app, err := Compile(App{
		ID: "inherited-cap", Name: "inherited-cap", Builtins: Builtins{Completion: true},
		Capabilities: []Capability{{ID: "disabled", Availability: AvailabilityUnavailable}},
		Root: Command{ID: "root", Name: "inherited-cap", Commands: []Command{{
			ID: "parent", Name: "parent", Capabilities: []CapabilityID{"disabled"}, Commands: []Command{{ID: "nested", Name: "nested"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, shell := range []Shell{ShellFish, ShellNushell} {
		script, err := app.CompletionScript(shell)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains([]byte(script), []byte("nested")) || bytes.Contains([]byte(script), []byte("parent")) {
			t.Fatalf("%s leaked unavailable inherited command:\n%s", shell, script)
		}
	}
	result, err := app.Complete(context.Background(), CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"inherited-cap", "p"}, CursorArg: 1, CursorOffset: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 0 {
		t.Fatalf("dynamic completion leaked inherited unavailable command: %+v", result.Candidates)
	}
}

func TestGeneratedArtifactsDeterministicRepeated(t *testing.T) {
	app := goldenFixture(t)
	generate := func() (map[string][]byte, error) {
		schema, err := app.SchemaJSON()
		if err != nil {
			return nil, err
		}
		contract, err := app.ContractJSON()
		if err != nil {
			return nil, err
		}
		out := map[string][]byte{
			"help":      []byte(app.Help()),
			"schema":    schema,
			"contract":  contract,
			"markdown":  []byte(app.MarkdownReference()),
			"man":       []byte(app.ManPage(1)),
			"help-nest": []byte(app.Help("serve")),
		}
		for _, shell := range SupportedShells() {
			script, err := app.CompletionScript(shell)
			if err != nil {
				return nil, err
			}
			out["shell-"+string(shell)] = []byte(script)
		}
		return out, nil
	}
	baseline, err := generate()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		got, err := generate()
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(baseline) {
			t.Fatalf("iteration %d artifact count changed", i)
		}
		for name, want := range baseline {
			if !bytes.Equal(got[name], want) {
				t.Fatalf("iteration %d artifact %s was nondeterministic", i, name)
			}
		}
	}
}

func ExampleConstraint_valuePredicate() {
	_ = Constraint{
		Kind: ValuePredicate, IDs: []string{"mode"}, PredicateID: "mode.safe",
		Validate: func(values ConstraintValues) error {
			mode, _ := ConstraintValueAs[string](values, "mode")
			if mode != "safe" {
				return fmt.Errorf("mode must be safe")
			}
			return nil
		},
	}
	// Output:
}

func TestHelpBuiltinIsPartOfCompiledGraph(t *testing.T) {
	app, err := Compile(App{ID: "help-graph", Name: "help-graph", Builtins: Builtins{Help: true}, Root: Command{ID: "root", Name: "help-graph", Commands: []Command{{ID: "child", Name: "child", Summary: "child command"}}}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, cmd := range app.Schema().Root.Commands {
		if cmd.ID == builtinHelpCommandID {
			found = true
		}
	}
	if !found {
		t.Fatal("help builtin missing from schema graph")
	}
	var out bytes.Buffer
	if err := app.Run(context.Background(), []string{"help", "child"}, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte("child command")) {
		t.Fatalf("help output=%q", out.String())
	}
	if err := app.Run(context.Background(), []string{"help", "missing"}, IO{Out: &out}); err == nil || ExitCode(err) != int(ExitUsage) {
		t.Fatalf("unknown help path err=%v", err)
	}
}

func TestCompileRejectsNamespaceAndNameCollisions(t *testing.T) {
	cases := []App{
		{ID: "reserved", Name: "reserved", Root: Command{ID: "root", Name: "reserved", Commands: []Command{{ID: "bad", Name: ReservedNamespace}}}},
		{ID: "dup-command", Name: "dup-command", Root: Command{ID: "root", Name: "dup-command", Commands: []Command{{ID: "one", Name: "same"}, {ID: "two", Name: "same"}}}},
		{ID: "alias-command", Name: "alias-command", Root: Command{ID: "root", Name: "alias-command", Commands: []Command{{ID: "one", Name: "one", Aliases: []string{"shared"}}, {ID: "two", Name: "shared"}}}},
		{ID: "dup-flag", Name: "dup-flag", Root: Command{ID: "root", Name: "dup-flag", Flags: []Flag{{ID: "one", Long: "same", Action: FlagSwitch}, {ID: "two", Long: "same", Action: FlagSwitch}}}},
		{ID: "alias-flag", Name: "alias-flag", Root: Command{ID: "root", Name: "alias-flag", Flags: []Flag{{ID: "one", Long: "one", Aliases: []string{"shared"}, Action: FlagSwitch}, {ID: "two", Long: "shared", Action: FlagSwitch}}}},
	}
	for i, app := range cases {
		if _, err := Compile(app); err == nil {
			t.Fatalf("case %d: expected compiler collision error", i)
		}
	}
}
