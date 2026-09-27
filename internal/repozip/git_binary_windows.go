//go:build windows

package repozip

// gitExecutableName deliberately names the native executable so os/exec cannot
// resolve a .cmd/.bat wrapper through PATHEXT. Git for Windows ships git.exe.
func gitExecutableName() string { return "git.exe" }
