package cli

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type grammarCapture struct {
	Verbose int64    `json:"verbose"`
	Name    string   `json:"name"`
	Tags    []string `json:"tags"`
	Input   string   `json:"input"`
	Rest    []string `json:"rest"`
}

func grammarApp(t *testing.T) *CompiledApp {
	t.Helper()
	app, err := Compile(App{ID: "grammar", Name: "grammar", Root: Command{
		ID: "root", Name: "grammar",
		Flags: []Flag{{ID: "verbose", Long: "verbose", Short: 'v', Action: FlagCount, Global: true}},
		Commands: []Command{{
			ID: "run", Name: "run", Aliases: []string{"r"}, OptionPolicy: OptionsInterspersed,
			Flags: []Flag{
				{ID: "name", Long: "name", Short: 'n', Value: StringValue()},
				{ID: "tag", Long: "tag", Short: 't', Value: StringValue(), Action: FlagAppend},
				{ID: "feature", Long: "feature", Action: FlagSwitch},
			},
			Args: []Arg{{ID: "input", Name: "input", Value: StringValue(), Required: true}, {ID: "rest", Name: "rest", Value: StringValue(), Mode: ArgVariadic}},
			Handler: func(inv *Invocation) error {
				verbose, _ := ValueAs[int64](inv, "verbose")
				name, _ := ValueAs[string](inv, "name")
				tags, _ := ValuesAs[string](inv, "tag")
				input, _ := ValueAs[string](inv, "input")
				rest, _ := ValuesAs[string](inv, "rest")
				return json.NewEncoder(inv.IO.Out).Encode(grammarCapture{Verbose: verbose, Name: name, Tags: tags, Input: input, Rest: rest})
			},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

func runGrammar(t *testing.T, app *CompiledApp, argv ...string) grammarCapture {
	t.Helper()
	var out byteBuffer
	if err := app.Run(context.Background(), argv, IO{Out: &out}); err != nil {
		t.Fatal(err)
	}
	var got grammarCapture
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

type byteBuffer struct{ b []byte }

func (b *byteBuffer) Write(p []byte) (int, error) { b.b = append(b.b, p...); return len(p), nil }
func (b *byteBuffer) Bytes() []byte               { return b.b }

func TestPublicGrammarMatrix(t *testing.T) {
	app := grammarApp(t)

	got := runGrammar(t, app, "-v", "r", "entrada", "--name=alice", "-t", "um", "--tag", "dois", "-v", "resto")
	if got.Verbose != 2 || got.Name != "alice" || len(got.Tags) != 2 || got.Tags[0] != "um" || got.Tags[1] != "dois" || got.Input != "entrada" || len(got.Rest) != 1 || got.Rest[0] != "resto" {
		t.Fatalf("interspersed/alias/global/repeat parse=%+v", got)
	}

	got = runGrammar(t, app, "run", "", "--", "--name", "值", "-v")
	if got.Input != "" || len(got.Rest) != 4 || got.Rest[0] != "--name" || got.Rest[1] != "值" || got.Rest[2] != "-v" {
		// The literal -- is a parser boundary, not part of argv delivered to positional values.
		if len(got.Rest) != 3 || got.Rest[0] != "--name" || got.Rest[1] != "值" || got.Rest[2] != "-v" {
			t.Fatalf("double-dash/empty/unicode parse=%+v", got)
		}
	}

	err := app.Run(context.Background(), []string{"run", "input", "--name"}, IO{})
	var d *Diagnostic
	if !errors.As(err, &d) || d.Code != CodeMissingValue {
		t.Fatalf("missing flag value err=%#v", err)
	}

	err = app.Run(context.Background(), []string{"run", "input", "--no-feature"}, IO{})
	if !errors.As(err, &d) || d.Code != CodeUnknownFlag {
		t.Fatalf("implicit bool negation unexpectedly accepted: %#v", err)
	}

	err = app.Run(context.Background(), []string{"run", "input", "-vt"}, IO{})
	if !errors.As(err, &d) || d.Code != CodeUnknownFlag {
		t.Fatalf("short cluster unexpectedly accepted: %#v", err)
	}
}
