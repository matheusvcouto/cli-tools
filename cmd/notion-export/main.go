package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"os"

	core "github.com/matheusvcouto/cli-tools/cli"
	notioncli "github.com/matheusvcouto/cli-tools/internal/notionexport/cli"
	"github.com/matheusvcouto/cli-tools/internal/version"
)

//go:embed tool.json
var toolManifestJSON []byte

func main() {
	ctx, stop := core.SignalContext(nil)
	defer stop()
	manifest, err := version.ParseToolManifest(toolManifestJSON, "notion-export")
	if err == nil {
		product := core.ProductMetadata{Version: manifest.Version, Stability: manifest.Stability, SuiteVersion: version.SuiteVersion}
		terminal := core.TerminalFromFiles(os.Stdin, os.Stdout, os.Stderr)
		interaction := core.TextInteraction{In: os.Stdin, Out: os.Stderr, Interactive: terminal.StdinTTY}
		app, compileErr := notioncli.New(notioncli.Dependencies{}, product, interaction)
		if compileErr == nil {
			err = app.Run(ctx, os.Args[1:], core.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Terminal: terminal})
		} else {
			err = compileErr
		}
	}
	if err != nil {
		if core.IsBrokenPipe(err) {
			return
		}
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "Cancelado. Nenhum export parcial foi publicado.")
			os.Exit(130)
		}
		core.RenderDiagnostic(os.Stderr, err)
		os.Exit(core.ExitCode(err))
	}
}
