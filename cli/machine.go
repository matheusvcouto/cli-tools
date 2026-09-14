package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
)

type doctorRow struct {
	ID      string       `json:"id"`
	Summary string       `json:"summary,omitempty"`
	Result  DoctorResult `json:"result"`
}

func (c *CompiledApp) runMachine(ctx context.Context, args []string, streams IO) error {
	if len(args) == 0 {
		return &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "invalid __cli request", Class: ExitUsage}
	}
	switch args[0] {
	case "schema":
		if len(args) != 1 {
			return invalidMachineUsage("schema")
		}
		if !c.builtins.Schema {
			return &Diagnostic{Code: CodeUnavailable, Kind: "protocol", Message: "schema endpoint is disabled", Class: ExitUnavailable}
		}
		b, err := c.SchemaJSON()
		if err != nil {
			return err
		}
		_, err = streams.Out.Write(b)
		return err
	case "contract":
		if len(args) != 1 {
			return invalidMachineUsage("contract")
		}
		b, err := c.ContractJSON()
		if err != nil {
			return err
		}
		_, err = streams.Out.Write(b)
		return err
	case "complete":
		if len(args) != 1 {
			return invalidMachineUsage("complete")
		}
		if !c.builtins.Completion {
			return &Diagnostic{Code: CodeUnavailable, Kind: "protocol", Message: "completion endpoint is disabled", Class: ExitUnavailable}
		}
		const maxCompletionRequestBytes = 1 << 20
		payload, err := io.ReadAll(io.LimitReader(streams.In, maxCompletionRequestBytes+1))
		if err != nil {
			return &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "read completion request: " + err.Error(), Class: ExitUsage, Cause: err}
		}
		if len(payload) > maxCompletionRequestBytes {
			return &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "completion request exceeds 1 MiB limit", Class: ExitUsage}
		}
		dec := json.NewDecoder(bytes.NewReader(payload))
		dec.DisallowUnknownFields()
		var req CompletionRequest
		if err := dec.Decode(&req); err != nil {
			return &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "invalid completion request: " + err.Error(), Class: ExitUsage, Cause: err}
		}
		var extra any
		if err := dec.Decode(&extra); err != io.EOF {
			return &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "completion request must contain exactly one JSON object", Class: ExitUsage}
		}
		res, err := c.Complete(ctx, req)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(streams.Out)
		enc.SetEscapeHTML(false)
		return enc.Encode(res)
	case "complete-shell":
		if !c.builtins.Completion {
			return &Diagnostic{Code: CodeUnavailable, Kind: "protocol", Message: "completion endpoint is disabled", Class: ExitUnavailable}
		}
		if len(args) < 6 || args[5] != "--" {
			return invalidMachineUsage("complete-shell")
		}
		protocol, err := strconv.Atoi(args[1])
		if err != nil {
			return invalidMachineUsage("complete-shell")
		}
		cursorArg, err := strconv.Atoi(args[3])
		if err != nil {
			return invalidMachineUsage("complete-shell")
		}
		cursorOffset, err := strconv.Atoi(args[4])
		if err != nil {
			return invalidMachineUsage("complete-shell")
		}
		res, err := c.Complete(ctx, CompletionRequest{Protocol: protocol, Shell: args[2], CursorArg: cursorArg, CursorOffset: cursorOffset, Argv: append([]string(nil), args[6:]...)})
		if err != nil {
			return err
		}
		return writeShellCompletion(streams.Out, res)
	default:
		return &Diagnostic{Code: CodeUnknownCommand, Kind: "protocol", Message: fmt.Sprintf("unknown __cli endpoint %q", args[0]), Class: ExitUsage}
	}
}

func invalidMachineUsage(endpoint string) error {
	return &Diagnostic{Code: CodeInvalidValue, Kind: "protocol", Message: "invalid __cli " + endpoint + " request", Class: ExitUsage}
}

func writeShellCompletion(w io.Writer, result CompletionResult) error {
	write := func(s string) error {
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
		_, err := io.WriteString(w, "\x00")
		return err
	}
	if err := write("cli-completion"); err != nil {
		return err
	}
	if err := write(strconv.Itoa(result.Protocol)); err != nil {
		return err
	}
	for _, candidate := range result.Candidates {
		for _, field := range []string{"candidate", candidate.Value, candidate.Label, candidate.Description, string(candidate.Kind), candidate.Group, candidate.ID} {
			if err := write(field); err != nil {
				return err
			}
		}
	}
	b := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	for _, field := range []string{"directive", b(result.Directive.Files), b(result.Directive.Directories), b(result.Directive.Executables), b(result.Directive.NoSpace), b(result.Directive.KeepOrder)} {
		if err := write(field); err != nil {
			return err
		}
	}
	return write("end")
}

func (c *CompiledApp) runDoctor(ctx context.Context, streams IO, jsonMode bool) error {
	rows := make([]doctorRow, 0, len(c.doctorChecks)+len(c.capabilities))
	for _, cap := range c.capabilities {
		status := DoctorUnsupported
		if cap.Availability == AvailabilityAvailable {
			status = DoctorHealthy
		}
		rows = append(rows, doctorRow{ID: "capability:" + string(cap.ID), Summary: cap.Summary, Result: DoctorResult{Status: status}})
	}
	for _, check := range c.doctorChecks {
		r := DoctorResult{Status: DoctorMissing, Message: "check unavailable"}
		if check.Check != nil {
			r = check.Check(ctx)
		}
		rows = append(rows, doctorRow{ID: check.ID, Summary: check.Summary, Result: r})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	if jsonMode {
		return json.NewEncoder(streams.Out).Encode(rows)
	}
	for _, r := range rows {
		if _, err := fmt.Fprintf(streams.Out, "%-12s %-28s %s\n", r.Result.Status, r.ID, r.Result.Message); err != nil {
			return err
		}
	}
	for _, r := range rows {
		if r.Result.Status == DoctorMissing || r.Result.Status == DoctorUnsupported {
			return &Diagnostic{Code: CodeRequirement, Kind: "doctor", Message: "one or more doctor checks are not healthy", Class: ExitUnavailable}
		}
	}
	return nil
}
