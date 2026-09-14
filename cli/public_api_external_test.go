package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	cli "github.com/matheusvcouto/cli-tools/cli"
)

func TestPublicAPIWorksFromExternalPackage(t *testing.T) {
	var handled string
	app, err := cli.Compile(cli.App{
		ID:       "example",
		Name:     "example",
		Summary:  "external package fixture",
		Product:  cli.ProductMetadata{Version: "1.0.0", Stability: "stable", SuiteVersion: "v1.0.0"},
		Builtins: cli.Builtins{Help: true, Version: true, Completion: true, Schema: true},
		Root: cli.Command{
			ID:   "example.root",
			Name: "example",
			Commands: []cli.Command{{
				ID:      "example.greet",
				Name:    "greet",
				Summary: "greet somebody",
				Args: []cli.Arg{{
					ID:       "example.greet.name",
					Name:     "name",
					Value:    cli.StringValue(),
					Required: true,
				}},
				Flags: []cli.Flag{{
					ID:      "example.greet.upper",
					Long:    "upper",
					Short:   'u',
					Action:  cli.FlagSwitch,
					Summary: "uppercase the greeting",
				}},
				Handler: func(inv *cli.Invocation) error {
					name, ok := cli.ValueAs[string](inv, "example.greet.name")
					if !ok {
						t.Fatal("typed positional value missing")
					}
					upper, _ := cli.ValueAs[bool](inv, "example.greet.upper")
					if upper {
						name = strings.ToUpper(name)
					}
					handled = name
					return nil
				},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := app.Run(context.Background(), []string{"greet", "Matheus", "--upper"}, cli.IO{Out: &stdout, Err: &stderr}); err != nil {
		t.Fatal(err)
	}
	if handled != "MATHEUS" {
		t.Fatalf("handler received %q", handled)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}

	result, err := app.Complete(context.Background(), cli.CompletionRequest{
		Protocol:     cli.CompletionProtocol,
		Argv:         []string{"gr"},
		CursorArg:    0,
		CursorOffset: 2,
		Shell:        string(cli.ShellBash),
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range result.Candidates {
		if candidate.Value == "greet" && candidate.Kind == cli.CandidateCommand {
			found = true
		}
	}
	if !found {
		t.Fatalf("completion did not expose greet: %+v", result.Candidates)
	}

	if got := app.Help(); !strings.Contains(got, "greet") {
		t.Fatalf("public Help output missing command: %q", got)
	}
	if schema := app.Schema(); schema.Version != cli.SchemaVersion {
		t.Fatalf("schema version=%d", schema.Version)
	}
	if contract := app.Contract(); contract.Version != cli.ContractVersion {
		t.Fatalf("contract version=%d", contract.Version)
	}
	if _, err := app.CompletionScript(cli.ShellBash); err != nil {
		t.Fatalf("public completion adapter: %v", err)
	}
}
