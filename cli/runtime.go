package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type CapabilityID string

type Availability uint8

const (
	AvailabilityUnknown Availability = iota
	AvailabilityAvailable
	AvailabilityUnavailable
)

type Capability struct {
	ID           CapabilityID
	Summary      string
	Availability Availability
}

type Terminal struct {
	StdinTTY  bool
	StdoutTTY bool
	StderrTTY bool
	Width     int
	Color     bool
}

// TerminalFromFiles detects stdio capabilities using only the standard library.
// Color is enabled only for a TTY and disabled by NO_COLOR or TERM=dumb.
// Width remains zero when it cannot be determined portably.
func TerminalFromFiles(in, out, errOut *os.File) Terminal {
	stdoutTTY := isCharDevice(out)
	_, noColor := os.LookupEnv("NO_COLOR")
	termDumb := strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb")
	return Terminal{
		StdinTTY:  isCharDevice(in),
		StdoutTTY: stdoutTTY,
		StderrTTY: isCharDevice(errOut),
		Color:     stdoutTTY && !noColor && !termDumb,
	}
}

func isCharDevice(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

type Interaction interface {
	Confirm(context.Context, Prompt) (bool, error)
	Text(context.Context, Prompt) (string, error)
	Secret(context.Context, Prompt) (string, error)
}

type Prompt struct {
	Message   string
	Default   string
	Dangerous bool
}

// TextInteraction is a minimal line-oriented interaction provider. It never
// prompts when Interactive is false.
type TextInteraction struct {
	In          io.Reader
	Out         io.Writer
	Interactive bool
	// SecretReader is intentionally injected: the standard library has no
	// portable no-echo terminal reader, and Secret must never silently echo.
	SecretReader func(context.Context, Prompt) (string, error)
}

func (p TextInteraction) ensure() error {
	if !p.Interactive {
		return &Diagnostic{Code: CodeNonInteractive, Kind: "interaction", Message: "interactive input is required", Class: ExitUnavailable}
	}
	if p.In == nil || p.Out == nil {
		return errors.New("interaction streams are not configured")
	}
	return nil
}
func (p TextInteraction) Confirm(ctx context.Context, prompt Prompt) (bool, error) {
	if err := p.ensure(); err != nil {
		return false, err
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	fmt.Fprint(p.Out, prompt.Message)
	if prompt.Default != "" {
		fmt.Fprintf(p.Out, " [%s]", prompt.Default)
	}
	fmt.Fprint(p.Out, " [y/N]: ")
	s, err := readInteractionLine(p.In)
	if err != nil {
		return false, err
	}
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		s = strings.ToLower(prompt.Default)
	}
	return s == "y" || s == "yes", nil
}
func (p TextInteraction) Text(ctx context.Context, prompt Prompt) (string, error) {
	if err := p.ensure(); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	fmt.Fprint(p.Out, prompt.Message)
	if prompt.Default != "" {
		fmt.Fprintf(p.Out, " [%s]", prompt.Default)
	}
	fmt.Fprint(p.Out, ": ")
	s, err := readInteractionLine(p.In)
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		s = prompt.Default
	}
	return s, nil
}
func (p TextInteraction) Secret(ctx context.Context, prompt Prompt) (string, error) {
	if err := p.ensure(); err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if p.SecretReader == nil {
		return "", &Diagnostic{Code: CodeUnavailable, Kind: "interaction", Message: "secure secret input is not available", Hint: "configure a no-echo secret reader for this platform", Class: ExitUnavailable}
	}
	return p.SecretReader(ctx, prompt)
}

// readInteractionLine intentionally performs no read-ahead. Interaction may
// issue multiple prompts against the same stream, so buffering a fresh reader
// per call could consume bytes that belong to the next prompt.
func readInteractionLine(r io.Reader) (string, error) {
	var b strings.Builder
	one := []byte{0}
	for {
		n, err := r.Read(one)
		if n > 0 {
			if one[0] == '\n' {
				return b.String(), nil
			}
			b.WriteByte(one[0])
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return b.String(), nil
			}
			return "", err
		}
		if n == 0 {
			return "", io.ErrNoProgress
		}
	}
}

type DoctorStatus string

const (
	DoctorHealthy     DoctorStatus = "healthy"
	DoctorWarning     DoctorStatus = "warning"
	DoctorMissing     DoctorStatus = "missing"
	DoctorUnsupported DoctorStatus = "unsupported"
)

type DoctorResult struct {
	Status  DoctorStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}
type DoctorCheck struct {
	ID      string
	Summary string
	Check   func(context.Context) DoctorResult
}
