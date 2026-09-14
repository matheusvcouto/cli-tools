package cli

import (
	"context"
	"os"
	"strings"
)

// ValueSource identifies where an effective value came from.
type ValueSource string

const (
	SourceCLI           ValueSource = "cli"
	SourceEnv           ValueSource = "env"
	SourceProjectConfig ValueSource = "project_config"
	SourceUserConfig    ValueSource = "user_config"
	SourceProvider      ValueSource = "provider"
	SourceDefault       ValueSource = "default"
)

// ResolutionProvider supplies textual values lazily during execution. Providers
// run only when the value was not supplied explicitly on the command line and
// are evaluated in declaration order. Resolve must be side-effect free with
// respect to command/domain state; reading environment/config is expected.
type ResolutionProvider struct {
	Source  ValueSource
	Name    string
	Resolve func(context.Context) (values []string, found bool, err error)
}

// EnvProvider resolves one value from an environment variable without reading
// it until an invocation actually needs that flag.
func EnvProvider(name string) ResolutionProvider {
	return ResolutionProvider{
		Source: SourceEnv,
		Name:   name,
		Resolve: func(context.Context) ([]string, bool, error) {
			value, ok := os.LookupEnv(name)
			if !ok {
				return nil, false, nil
			}
			return []string{value}, true, nil
		},
	}
}

func validProviderSource(source ValueSource) bool {
	switch source {
	case SourceEnv, SourceProjectConfig, SourceUserConfig, SourceProvider:
		return true
	default:
		return false
	}
}

func providerKey(p ResolutionProvider) string {
	return string(p.Source) + "\x00" + strings.TrimSpace(p.Name)
}
