package cli

import "context"

const CompletionProtocol = 1

const MaxCompletionCandidates = 256

type CandidateKind string

const (
	CandidateCommand CandidateKind = "command"
	CandidateFlag    CandidateKind = "flag"
	CandidateValue   CandidateKind = "value"
)

type CompletionCandidate struct {
	Value       string        `json:"value"`
	Label       string        `json:"label,omitempty"`
	Description string        `json:"description,omitempty"`
	Kind        CandidateKind `json:"kind,omitempty"`
	Group       string        `json:"group,omitempty"`
	ID          string        `json:"id,omitempty"`
}

type CompletionDirective struct {
	Files       bool `json:"files,omitempty"`
	Directories bool `json:"directories,omitempty"`
	Executables bool `json:"executables,omitempty"`
	NoSpace     bool `json:"no_space,omitempty"`
	KeepOrder   bool `json:"keep_order,omitempty"`
}

type CompletionRequest struct {
	Protocol     int      `json:"protocol"`
	Argv         []string `json:"argv"`
	CursorArg    int      `json:"cursor_arg"`
	CursorOffset int      `json:"cursor_offset"`
	Shell        string   `json:"shell,omitempty"`
}

type CompletionResult struct {
	Protocol   int                   `json:"protocol"`
	Candidates []CompletionCandidate `json:"candidates"`
	Directive  CompletionDirective   `json:"directive"`
}

type CompleteContext struct {
	CommandID string
	ArgID     string
	FlagID    string
	Prefix    string
	values    map[string][]any
}

// CompletionValueAs returns the last previously parsed, non-sensitive value for
// a stable ID. Values are parsed once by the core; completers never need to
// reinterpret argv.
func CompletionValueAs[T any](c CompleteContext, id string) (T, bool) {
	var zero T
	values := c.values[id]
	if len(values) == 0 {
		return zero, false
	}
	v, ok := values[len(values)-1].(T)
	return v, ok
}

// CompletionValuesAs returns all previously parsed, non-sensitive values for a
// stable ID.
func CompletionValuesAs[T any](c CompleteContext, id string) ([]T, bool) {
	values := c.values[id]
	out := make([]T, 0, len(values))
	for _, value := range values {
		v, ok := value.(T)
		if !ok {
			return nil, false
		}
		out = append(out, v)
	}
	return out, len(out) > 0
}

type Completer func(context.Context, CompleteContext) ([]CompletionCandidate, error)
