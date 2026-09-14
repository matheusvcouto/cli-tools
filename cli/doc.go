// Package cli provides a declarative, compiled command-line application model.
//
// Applications are described once, compiled into an immutable graph, then used
// for parsing, execution, help, diagnostics and completion. The package never
// reads os.Args, exits the process, or installs signal handlers implicitly.
package cli
