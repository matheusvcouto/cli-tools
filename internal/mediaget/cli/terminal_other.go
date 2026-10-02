//go:build !darwin && !linux

package mediacli

import (
	"context"
	"os"
)

func nativeTerminalAvailable() bool                             { return false }
func readTerminalKey(context.Context, *os.File) (string, error) { return "", errTextSelect }
