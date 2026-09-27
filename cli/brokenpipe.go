package cli

import (
	"errors"
	"io"
)

// IsBrokenPipe reports whether an output failure means the downstream consumer
// closed a pipe early. Composition roots can treat this as a normal pipeline
// termination instead of rendering an internal-error diagnostic.
func IsBrokenPipe(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) || isPlatformBrokenPipe(err)
}
