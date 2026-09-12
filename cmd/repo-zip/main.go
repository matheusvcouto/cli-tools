package main

import (
	"context"
	"os"

	"github.com/matheusvcouto/cli-tools/internal/cliapp"
	"github.com/matheusvcouto/cli-tools/internal/repozip"
	"github.com/matheusvcouto/cli-tools/internal/version"
)

func main() {
	app := repozip.App{Service: repozip.Service{Git: repozip.Git{}, Archiver: repozip.Archiver{}}, Version: version.Version}
	if err := app.Run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		cliapp.RenderError(os.Stderr, err)
		os.Exit(cliapp.ExitCode(err))
	}
}
