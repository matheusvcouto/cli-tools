package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget"
	mediacli "github.com/matheusvcouto/cli-tools/internal/mediaget/cli"
	"github.com/matheusvcouto/cli-tools/internal/mediaget/ytdlp"
	"github.com/matheusvcouto/cli-tools/internal/version"
)

//go:embed tool.json
var toolManifestJSON []byte

func main() {
	ctx, stop := core.SignalContext(nil)
	defer stop()
	manifest, err := version.ParseToolManifest(toolManifestJSON, "media-get")
	if err == nil {
		product := core.ProductMetadata{Version: manifest.Version, Stability: manifest.Stability, SuiteVersion: version.SuiteVersion}
		streams, interaction := mediacli.TerminalIO(os.Stdin, os.Stdout, os.Stderr)
		app, compileErr := mediacli.New(mediaget.Service{Backend: ytdlp.Adapter{}}, product, interaction)
		if compileErr == nil {
			err = app.Run(ctx, os.Args[1:], streams)
		} else {
			err = compileErr
		}
	}
	if err != nil {
		if core.IsBrokenPipe(err) {
			return
		}
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, mediacli.CancellationMessage(err))
			os.Exit(130)
		}
		core.RenderDiagnostic(os.Stderr, err)
		os.Exit(core.ExitCode(err))
	}
}
