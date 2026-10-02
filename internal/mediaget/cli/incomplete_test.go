package mediacli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
)

type incompleteBackend struct{}

func (incompleteBackend) Check(context.Context, mediaget.Selection) error { return nil }
func (incompleteBackend) Inspect(context.Context, mediaget.Source, mediaget.Selection) (mediaget.Info, error) {
	return mediaget.Info{}, nil
}
func (incompleteBackend) Download(_ context.Context, _ mediaget.Request, dir string, _ func(mediaget.Progress)) error {
	if err := os.WriteFile(filepath.Join(dir, "partial.part"), []byte("partial"), 0600); err != nil {
		return err
	}
	return context.Canceled
}

func (c cleanupInteraction) Confirm(context.Context, core.Prompt) (bool, error) {
	return false, errors.New("unexpected confirm")
}

func (c cleanupInteraction) Secret(context.Context, core.Prompt) (string, error) {
	return "", errors.New("unexpected secret")
}

type cleanupInteraction struct {
	choice int
	err    error
}

func (c cleanupInteraction) Text(context.Context, core.Prompt) (string, error) {
	return "", errors.New("unexpected text prompt")
}
func (c cleanupInteraction) Select(ctx context.Context, _ string, _ []string, _ bool) (int, error) {
	if ctx.Err() != nil {
		return -1, errors.New("cleanup reused canceled context")
	}
	return c.choice, c.err
}

func TestIncompleteDecisionAfterCanceledTransfer(t *testing.T) {
	for _, test := range []struct {
		name   string
		choice int
		err    error
		keep   bool
	}{
		{"discard default", 0, nil, false}, {"keep", 1, nil, true}, {"second interrupt", -1, context.Canceled, false}, {"EOF", -1, errors.New("EOF"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := mediaget.Request{Source: mediaget.Source{URL: "https://example.invalid/media"}, Selection: mediaget.Selection{Kind: mediaget.Video}, OutputDir: t.TempDir(), KeepIncomplete: true}
			result, cause := (mediaget.Service{Backend: incompleteBackend{}}).Download(context.Background(), req, nil)
			if result.Incomplete == nil {
				t.Fatal("no partial")
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			var output bytes.Buffer
			inv := &core.Invocation{Context: ctx, IO: core.IO{Err: &output}, Interaction: cleanupInteraction{choice: test.choice, err: test.err}}
			err := finishIncomplete(inv, result.Incomplete, cause)
			if !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			_, statErr := os.Lstat(result.Incomplete.Path)
			if test.keep && statErr != nil || !test.keep && !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("keep=%t %v", test.keep, statErr)
			}
			if !strings.Contains(output.String(), "7 B") || !strings.Contains(output.String(), result.Incomplete.Path) {
				t.Fatal(output.String())
			}
		})
	}
}
