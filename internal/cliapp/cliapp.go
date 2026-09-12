package cliapp

import (
	"errors"
	"fmt"
	"io"
)

// ExitCoder lets a command opt into a non-default exit status without making
// cliapp own a command framework or error hierarchy.
type ExitCoder interface {
	ExitCode() int
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var coded ExitCoder
	if errors.As(err, &coded) && coded.ExitCode() > 0 {
		return coded.ExitCode()
	}
	return 1
}

func RenderError(w io.Writer, err error) {
	if err != nil {
		fmt.Fprintf(w, "error: %v\n", err)
	}
}
