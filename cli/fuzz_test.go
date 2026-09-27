package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	iparse "github.com/matheusvcouto/cli-tools/cli/internal/parse"
)

func fuzzFixture(f *testing.F) *CompiledApp {
	f.Helper()
	c, err := Compile(App{ID: "f", Name: "f", Builtins: Builtins{Completion: true}, Root: Command{
		ID: "root", Name: "f",
		Args:  []Arg{{ID: "arg", Name: "arg", Value: StringValue(), Mode: ArgVariadic}},
		Flags: []Flag{{ID: "n", Long: "number", Short: 'n', Value: IntValue()}},
	}})
	if err != nil {
		f.Fatal(err)
	}
	return c
}

func FuzzStrictAndPartialNeverPanic(f *testing.F) {
	c := fuzzFixture(f)
	for _, s := range []string{"", "--number", "--number=1", "--", "-n", "abc", "-1", "--\x1f-x"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, encoded string) {
		argv := strings.Split(encoded, "\x1f")
		_, strictErr := iparse.Run(c.graph.Root, argv, iparse.Strict)
		_, partialErr := iparse.Run(c.graph.Root, argv, iparse.Partial)
		if strictErr == nil && partialErr != nil {
			t.Fatalf("strict accepted argv but partial rejected it: argv=%q err=%v", argv, partialErr)
		}
		_ = c.Run(context.Background(), argv, IO{})
		_, _ = c.Complete(context.Background(), CompletionRequest{Protocol: 1, Argv: append([]string{"f"}, argv...), CursorArg: len(argv), CursorOffset: 0})
	})
}

func FuzzCompletionProtocolDecoderNeverPanics(f *testing.F) {
	app, err := Compile(App{ID: "protocol-fuzz", Name: "protocol-fuzz", Builtins: Builtins{Completion: true}, Root: Command{ID: "root", Name: "protocol-fuzz"}})
	if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{``, `{}`, `{"protocol":1,"argv":["protocol-fuzz"],"cursor_arg":0,"cursor_offset":0}`, `{"protocol":99,"argv":[],"cursor_arg":-1}`, `{"protocol":1,"argv":["olá"],"cursor_arg":0,"unknown":true}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_ = app.Run(context.Background(), []string{"__cli", "complete"}, IO{In: strings.NewReader(input), Out: io.Discard, Err: io.Discard})
	})
}

func FuzzSchemaAndContractJSONRoundTrip(f *testing.F) {
	for _, seed := range []string{`{}`, `{"schema_version":1,"app_id":"x","name":"x","root":{"id":"root","name":"x"}}`, `null`, `{"contract_version":1,"root":{"id":"root","name":"x"}}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		canonicalStable := func(dst any, next func() any) {
			if err := json.Unmarshal([]byte(input), dst); err != nil {
				return
			}
			first, err := json.Marshal(dst)
			if err != nil {
				t.Fatal(err)
			}
			secondValue := next()
			if err := json.Unmarshal(first, secondValue); err != nil {
				t.Fatal(err)
			}
			second, err := json.Marshal(secondValue)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatalf("typed JSON normalization is not stable: %s != %s", first, second)
			}
		}
		canonicalStable(&Schema{}, func() any { return &Schema{} })
		canonicalStable(&Contract{}, func() any { return &Contract{} })
	})
}

func FuzzShellEscapersNeverPanic(f *testing.F) {
	for _, seed := range []string{"", "plain", "two words", "'\"$`();[]", "olá 世界", "line\nnext", "-leading"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_ = shSingleQuote(input)
		_ = psSingleQuote(input)
		_ = fishQuote(input)
		_ = nuExternal(input)
		_ = shellIdentifier(input)
	})
}
