package cli

import (
	"context"
	"fmt"
	"io"
	"testing"

	iparse "github.com/matheusvcouto/cli-tools/cli/internal/parse"
)

func benchmarkSpec(commands, flags int) App {
	children := make([]Command, 0, commands)
	for i := 0; i < commands; i++ {
		fs := make([]Flag, 0, flags)
		for j := 0; j < flags; j++ {
			fs = append(fs, Flag{
				ID:      fmt.Sprintf("cmd.%d.flag.%d", i, j),
				Long:    fmt.Sprintf("flag-%d", j),
				Summary: "benchmark flag",
				Value:   StringValue(),
			})
		}
		children = append(children, Command{
			ID:      fmt.Sprintf("cmd.%d", i),
			Name:    fmt.Sprintf("cmd-%d", i),
			Summary: "benchmark command",
			Args: []Arg{{
				ID:       fmt.Sprintf("cmd.%d.arg", i),
				Name:     "target",
				Value:    StringValue(),
				Required: true,
			}},
			Flags:   fs,
			Handler: func(*Invocation) error { return nil },
		})
	}
	return App{ID: "benchmark", Name: "bench", Root: Command{ID: "root", Name: "bench", Commands: children}}
}

func BenchmarkCompile(b *testing.B) {
	for _, tc := range []struct {
		name     string
		commands int
		flags    int
	}{{"small", 3, 3}, {"medium", 20, 8}, {"large", 80, 12}} {
		b.Run(tc.name, func(b *testing.B) {
			spec := benchmarkSpec(tc.commands, tc.flags)
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Compile(spec); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkCompiled(b *testing.B) *CompiledApp {
	b.Helper()
	spec := benchmarkSpec(20, 8)
	spec.Root.Commands[0].Args[0].Completer = func(context.Context, CompleteContext) ([]CompletionCandidate, error) {
		return []CompletionCandidate{{Value: "alpha", Kind: CandidateValue}, {Value: "beta", Kind: CandidateValue}}, nil
	}
	app, err := Compile(spec)
	if err != nil {
		b.Fatal(err)
	}
	return app
}

func BenchmarkStrictParseAndBind(b *testing.B) {
	app := benchmarkCompiled(b)
	argv := []string{"cmd-0", "target", "--flag-0", "value", "--flag-7=tail"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := app.Run(context.Background(), argv, IO{Out: io.Discard, Err: io.Discard}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPartialParse(b *testing.B) {
	app := benchmarkCompiled(b)
	argv := []string{"cmd-0", "target", "--fl"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := iparse.Run(app.graph.Root, argv, iparse.Partial); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPartialStaticCompletion(b *testing.B) {
	app := benchmarkCompiled(b)
	req := CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"bench", "cmd-0", "--fl"}, CursorArg: 2, CursorOffset: 4}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := app.Complete(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDynamicCompletionDispatch(b *testing.B) {
	app := benchmarkCompiled(b)
	req := CompletionRequest{Protocol: CompletionProtocol, Argv: []string{"bench", "cmd-0", "a"}, CursorArg: 2, CursorOffset: 1}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := app.Complete(context.Background(), req); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHelpRender(b *testing.B) {
	app := benchmarkCompiled(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = app.Help("cmd-0")
	}
}

func BenchmarkSchemaAndContractGeneration(b *testing.B) {
	app := benchmarkCompiled(b)
	b.Run("schema", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := app.SchemaJSON(); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("contract", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := app.ContractJSON(); err != nil {
				b.Fatal(err)
			}
		}
	})
}
