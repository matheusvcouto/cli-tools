package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var updateGolden = flag.Bool("update-golden", false, "rewrite CLI generated-artifact golden files")

func goldenFixture(t *testing.T) *CompiledApp {
	t.Helper()
	app, err := Compile(App{
		ID:          "golden",
		Name:        "golden-cli",
		Summary:     "Golden fixture",
		Description: "Deterministic generated artifact fixture.",
		Product:     ProductMetadata{Version: "1.2.3", Stability: "beta", SuiteVersion: "4.5.6"},
		Builtins:    Builtins{Help: true, Version: true, Completion: true, Schema: true},
		Root: Command{ID: "root", Name: "golden-cli", Flags: []Flag{
			{ID: "verbose", Long: "verbose", Short: 'v', Summary: "increase verbosity", Action: FlagCount, Global: true},
		}, Commands: []Command{{
			ID: "serve", Name: "serve", Aliases: []string{"s"}, Summary: "serve a target", Description: "Serve one target using the selected format.",
			Args:  []Arg{{ID: "serve.target", Name: "target", Summary: "target name", Value: EnumValue(Choice{Value: "alpha", Description: "first target"}, Choice{Value: "two words", Description: "spaced target"}), Required: true}},
			Flags: []Flag{{ID: "serve.format", Long: "format", Short: 'f', Summary: "output format", Value: EnumValue(Choice{Value: "json"}, Choice{Value: "text"}), Default: []string{"text"}}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func TestGeneratedArtifactsGolden(t *testing.T) {
	app := goldenFixture(t)
	schema, err := app.SchemaJSON()
	if err != nil {
		t.Fatal(err)
	}
	contract, err := app.ContractJSON()
	if err != nil {
		t.Fatal(err)
	}
	artifacts := map[string][]byte{
		"help-root.txt":  []byte(app.Help()),
		"help-serve.txt": []byte(app.Help("serve")),
		"schema.json":    schema,
		"contract.json":  contract,
		"reference.md":   []byte(app.MarkdownReference()),
		"golden-cli.1":   []byte(app.ManPage(1)),
	}
	for _, shell := range SupportedShells() {
		script, err := app.CompletionScript(shell)
		if err != nil {
			t.Fatal(err)
		}
		artifacts["completion-"+string(shell)+".txt"] = []byte(script)
	}

	dir := filepath.Join("testdata", "golden")
	if *updateGolden {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, got := range artifacts {
		path := filepath.Join(dir, name)
		if *updateGolden {
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v (regenerate with: go test ./cli -run TestGeneratedArtifactsGolden -args -update-golden)", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("generated artifact %s changed; inspect and regenerate explicitly with: go test ./cli -run TestGeneratedArtifactsGolden -args -update-golden", path)
		}
	}
}
