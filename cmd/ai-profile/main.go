package main

import (
	_ "embed"
	"os"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/aiprofile"
	profilecli "github.com/matheusvcouto/cli-tools/internal/aiprofile/cli"
	"github.com/matheusvcouto/cli-tools/internal/aiprofile/platform"
	"github.com/matheusvcouto/cli-tools/internal/version"
)

//go:embed tool.json
var toolManifestJSON []byte

func main() {
	ctx, stop := core.SignalContext(nil)
	defer stop()

	manifest, err := version.ParseToolManifest(toolManifestJSON, "ai-profile")
	if err != nil {
		core.RenderDiagnostic(os.Stderr, err)
		os.Exit(core.ExitCode(err))
	}
	product := core.ProductMetadata{Version: manifest.Version, Stability: manifest.Stability, SuiteVersion: version.SuiteVersion}

	service := core.NewLazy(func() (*aiprofile.Service, error) {
		root, err := aiprofile.DefaultRoot()
		if err != nil {
			return nil, err
		}
		return aiprofile.NewService(aiprofile.Store{Root: root}, platform.Runner{})
	})
	app, err := profilecli.New(service, aiprofile.ProcessIO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}, product)
	if err == nil {
		err = app.Run(ctx, os.Args[1:], core.IO{In: os.Stdin, Out: os.Stdout, Err: os.Stderr, Terminal: core.TerminalFromFiles(os.Stdin, os.Stdout, os.Stderr)})
	}
	if err != nil {
		if core.IsBrokenPipe(err) {
			return
		}
		core.RenderDiagnostic(os.Stderr, err)
		os.Exit(core.ExitCode(err))
	}
}
