package main

import (
	_ "embed"
	"os"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/repozip"
	repocli "github.com/matheusvcouto/cli-tools/internal/repozip/cli"
	"github.com/matheusvcouto/cli-tools/internal/version"
)

//go:embed tool.json
var toolManifestJSON []byte

func main() {
	ctx, stop := core.SignalContext(nil)
	defer stop()

	manifest, err := version.ParseToolManifest(toolManifestJSON, "repo-zip")
	if err == nil {
		product := core.ProductMetadata{Version: manifest.Version, Stability: manifest.Stability, SuiteVersion: version.SuiteVersion}
		app, compileErr := repocli.New(repozip.Service{Git: repozip.Git{}, Archiver: repozip.Archiver{}}, product)
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
		os.Exit(core.ExitCode(err))
	}
}
