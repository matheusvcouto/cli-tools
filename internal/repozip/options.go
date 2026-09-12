package repozip

import (
	"fmt"
	"path/filepath"
	"strings"
)

const DefaultOutputDir = ".tmp/repo-zip"

type Options struct {
	Source  string
	Output  string
	Name    string
	Git     bool
	Suffix  string
	Version string
	Force   bool
}

func ParseArgs(args []string) (Options, error) {
	opts := Options{Source: "."}
	var sourceSet bool
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			for _, rest := range args[i+1:] {
				if sourceSet {
					return Options{}, fmt.Errorf("only one source directory may be specified")
				}
				opts.Source, sourceSet = rest, true
			}
			break
		}
		take := func(name string) (string, error) {
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s requires a value", name)
			}
			i++
			return args[i], nil
		}
		switch a {
		case "-o", "--output":
			v, err := take(a)
			if err != nil {
				return Options{}, err
			}
			opts.Output = v
		case "-n", "--name":
			v, err := take(a)
			if err != nil {
				return Options{}, err
			}
			opts.Name = v
		case "--git":
			opts.Git = true
		case "-s", "--suffix":
			v, err := take(a)
			if err != nil {
				return Options{}, err
			}
			opts.Suffix = v
		case "-v", "--version":
			v, err := take(a)
			if err != nil {
				return Options{}, err
			}
			opts.Version = v
		case "-f", "--force":
			opts.Force = true
		case "-h", "--help":
			return Options{}, ErrHelp
		default:
			if strings.HasPrefix(a, "-") {
				return Options{}, fmt.Errorf("unknown option %q", a)
			}
			if sourceSet {
				return Options{}, fmt.Errorf("only one source directory may be specified")
			}
			opts.Source, sourceSet = a, true
		}
	}
	if opts.Output != "" && opts.Name != "" {
		return Options{}, fmt.Errorf("use --output or --name, not both")
	}
	if opts.Suffix != "" && opts.Version != "" {
		return Options{}, fmt.Errorf("use --suffix or --version, not both")
	}
	if opts.Output != "" && (opts.Suffix != "" || opts.Version != "") {
		return Options{}, fmt.Errorf("--output defines the exact filename; include any suffix in that path")
	}
	return opts, nil
}

func normalizeBaseName(value string) (string, error) {
	checked, err := validateComponent(value, "--name")
	if err != nil {
		return "", err
	}
	if strings.EqualFold(filepath.Ext(checked), ".zip") {
		checked = strings.TrimSuffix(checked, filepath.Ext(checked))
	}
	return validateComponent(checked, "--name")
}

func validateSuffix(value, label string) (string, error) {
	checked, err := validateComponent(value, label)
	if err != nil {
		return "", err
	}
	for i, r := range checked {
		valid := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if !valid || (i == 0 && (r == '.' || r == '_' || r == '-')) {
			return "", fmt.Errorf("%s accepts only letters, numbers, dot, _ and -, and must start alphanumeric", label)
		}
	}
	return checked, nil
}

func validateComponent(value, label string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", label)
	}
	if strings.TrimSpace(value) != value {
		return "", fmt.Errorf("%s cannot start or end with whitespace", label)
	}
	if strings.ContainsAny(value, `/\\`) {
		return "", fmt.Errorf("%s must be a filename component, not a path", label)
	}
	if value == "." || value == ".." {
		return "", fmt.Errorf("invalid %s: %q", label, value)
	}
	return value, nil
}
