package repozip

import (
	"fmt"
	"path/filepath"
	"strings"
)

const DefaultOutputDir = ".tmp/repo-zip"

type Options struct {
	Source string
	Output string
	Name   string
	Git    bool
	Suffix string
	Force  bool
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
