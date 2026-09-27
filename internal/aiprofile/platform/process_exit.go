package platform

import "errors"

type childExitCoder interface {
	childExitCode() int
}

// ChildExitCode reports an exit status produced by a child that was started
// successfully. The composition root uses this to preserve the child's status
// without printing a wrapper diagnostic, which is especially important for ACP.
func ChildExitCode(err error) (int, bool) {
	var childExit childExitCoder
	if !errors.As(err, &childExit) {
		return 0, false
	}
	return childExit.childExitCode(), true
}
