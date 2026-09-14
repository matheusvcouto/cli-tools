package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type ExitClass int

const (
	ExitSuccess     ExitClass = 0
	ExitExecution   ExitClass = 1
	ExitUsage       ExitClass = 2
	ExitUnavailable ExitClass = 3
)

type DiagnosticCode string

const (
	CodeUnknownCommand  DiagnosticCode = "cli.unknown_command"
	CodeUnknownFlag     DiagnosticCode = "cli.unknown_flag"
	CodeMissingValue    DiagnosticCode = "cli.missing_value"
	CodeMissingArgument DiagnosticCode = "cli.missing_argument"
	CodeInvalidValue    DiagnosticCode = "cli.invalid_value"
	CodeConstraint      DiagnosticCode = "cli.constraint"
	CodeUnavailable     DiagnosticCode = "cli.unavailable"
	CodeRequirement     DiagnosticCode = "cli.requirement"
	CodeNonInteractive  DiagnosticCode = "cli.non_interactive"
	CodeResolution      DiagnosticCode = "cli.resolution"
	CodeInternal        DiagnosticCode = "cli.internal"
)

type Diagnostic struct {
	Code        DiagnosticCode `json:"code"`
	Kind        string         `json:"kind,omitempty"`
	Message     string         `json:"message"`
	Hint        string         `json:"hint,omitempty"`
	CommandPath []string       `json:"command_path,omitempty"`
	RelatedID   string         `json:"related_id,omitempty"`
	Class       ExitClass      `json:"-"`
	Cause       error          `json:"-"`
	Sensitive   bool           `json:"-"`
}

func (d *Diagnostic) Error() string { return d.Message }
func (d *Diagnostic) Unwrap() error { return d.Cause }
func (d *Diagnostic) ExitCode() int {
	if d == nil {
		return 0
	}
	return int(d.Class)
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var x interface{ ExitCode() int }
	if errors.As(err, &x) && x.ExitCode() > 0 {
		return x.ExitCode()
	}
	return int(ExitExecution)
}

type diagnosticView struct {
	Code        DiagnosticCode `json:"code"`
	Kind        string         `json:"kind,omitempty"`
	Message     string         `json:"message"`
	Hint        string         `json:"hint,omitempty"`
	CommandPath []string       `json:"command_path,omitempty"`
	RelatedID   string         `json:"related_id,omitempty"`
	ExitCode    int            `json:"exit_code"`
}

func normalizedDiagnostic(err error) diagnosticView {
	var d *Diagnostic
	if errors.As(err, &d) {
		message, hint := d.Message, d.Hint
		if d.Sensitive {
			message = "sensitive diagnostic redacted"
			hint = ""
		}
		return diagnosticView{Code: d.Code, Kind: d.Kind, Message: message, Hint: hint, CommandPath: append([]string(nil), d.CommandPath...), RelatedID: d.RelatedID, ExitCode: d.ExitCode()}
	}
	return diagnosticView{Code: CodeInternal, Kind: "internal", Message: err.Error(), ExitCode: int(ExitExecution)}
}

// RenderDiagnostic writes the stable human diagnostic form. Sensitive
// diagnostics never expose their original message or hint.
func RenderDiagnostic(w io.Writer, err error) {
	if err == nil {
		return
	}
	v := normalizedDiagnostic(err)
	fmt.Fprintf(w, "error: %s\n", v.Message)
	if v.Hint != "" {
		fmt.Fprintf(w, "hint: %s\n", v.Hint)
	}
}

// RenderDiagnosticJSON writes the same structured diagnostic as one JSON value.
// Wrapped causes are deliberately excluded from this public representation.
func RenderDiagnosticJSON(w io.Writer, err error) error {
	if err == nil {
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(normalizedDiagnostic(err))
}
