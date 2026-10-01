package main

import (
	"context"
	_ "embed"
	"errors"
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
		app, compileErr := mediacli.New(mediaget.Service{Backend: ytdlp.Adapter{}}, product)
		if compileErr == nil {
			err = app.Run(ctx, os.Args[1:], core.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Terminal: core.TerminalFromFiles(os.Stdin, os.Stdout, os.Stderr)})
		} else {
			err = compileErr
		}
	}
	if err != nil {
		if core.IsBrokenPipe(err) {
			return
		}
		core.RenderDiagnostic(os.Stderr, err)
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		os.Exit(core.ExitCode(err))
	}
}
