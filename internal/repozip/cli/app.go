package repocli

import (
	"fmt"

	core "github.com/matheusvcouto/cli-tools/cli"
	"github.com/matheusvcouto/cli-tools/internal/repozip"
)

const (
	argSource  = "repozip.source"
	flagOutput = "repozip.output"
	flagName   = "repozip.name"
	flagGit    = "repozip.git"
	flagSuffix = "repozip.suffix"
	flagForce  = "repozip.force"
)

// New compiles the repo-zip CLI specification. The service is captured only by
// the execution handler, so help/version/schema/completion remain side-effect free.
func New(service repozip.Service, product core.ProductMetadata) (*core.CompiledApp, error) {
	app := core.App{
		ID:       "repo-zip",
		Name:     "repo-zip",
		Summary:  "create a Git-aware ZIP snapshot",
		Product:  product,
		Builtins: core.Builtins{Help: true, Version: true, Completion: true, Schema: true},
		Root: core.Command{
			ID:           "repozip.root",
			Name:         "repo-zip",
			OptionPolicy: core.OptionsInterspersed,
			Args:         []core.Arg{{ID: argSource, Name: "source", Summary: "repository directory", Value: core.DirectoryValue()}},
			Flags: []core.Flag{
				{ID: flagOutput, Long: "output", Short: 'o', Summary: "exact output path (.zip)", Value: core.FileValue()},
				{ID: flagName, Long: "name", Short: 'n', Summary: "base name for automatic output", Value: core.StringValue()},
				{ID: flagGit, Long: "git", Summary: "include restorable Git bundle and Git state in automatic name", Action: core.FlagSwitch},
				{ID: flagSuffix, Long: "suffix", Short: 's', Summary: "append suffix to automatic name", Value: core.StringValue()},
				{ID: flagForce, Long: "force", Short: 'f', Summary: "atomically replace an existing regular output file", Action: core.FlagSwitch},
			},
			Constraints: []core.Constraint{
				{Kind: core.Conflicts, IDs: []string{flagOutput, flagName}, Message: "use --output or --name, not both"},
				{Kind: core.Conflicts, IDs: []string{flagOutput, flagSuffix}, Message: "--output defines the exact filename; include any suffix in that path"},
			},
			Handler: func(inv *core.Invocation) error {
				opts := repozip.Options{Source: "."}
				if source, ok := core.ValueAs[string](inv, argSource); ok {
					opts.Source = source
				}
				if value, ok := core.ValueAs[string](inv, flagOutput); ok {
					opts.Output = value
				}
				if value, ok := core.ValueAs[string](inv, flagName); ok {
					opts.Name = value
				}
				if value, ok := core.ValueAs[string](inv, flagSuffix); ok {
					opts.Suffix = value
				}
				if value, ok := core.ValueAs[bool](inv, flagGit); ok {
					opts.Git = value
				}
				if value, ok := core.ValueAs[bool](inv, flagForce); ok {
					opts.Force = value
				}
				result, err := service.Run(inv.Context, opts)
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(inv.IO.Out, result.Output)
				return err
			},
		},
	}
	return core.Compile(app)
}
