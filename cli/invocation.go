package cli

import (
	"context"
	"io"
)

type IO struct {
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
	Terminal Terminal
}

type ResolvedValue struct {
	Value      any
	Source     ValueSource
	SourceName string
}

type Invocation struct {
	Context      context.Context
	IO           IO
	CommandID    string
	CommandPath  []string
	Terminal     Terminal
	Interaction  Interaction
	raw          map[string][]ResolvedValue
	capabilities map[CapabilityID]Capability
}

// Capability returns composition-root capability metadata without exposing the
// mutable compiler/runtime internals. Domain services remain explicit handler
// dependencies rather than a generic service locator.
func (i *Invocation) Capability(id CapabilityID) (Capability, bool) {
	cap, ok := i.capabilities[id]
	return cap, ok
}

func (i *Invocation) Present(id string) bool { return len(i.raw[id]) > 0 }
func (i *Invocation) Values(id string) []ResolvedValue {
	return append([]ResolvedValue(nil), i.raw[id]...)
}
func ValueAs[T any](i *Invocation, id string) (T, bool) {
	var zero T
	values := i.raw[id]
	if len(values) == 0 {
		return zero, false
	}
	v, ok := values[len(values)-1].Value.(T)
	return v, ok
}
func ValuesAs[T any](i *Invocation, id string) ([]T, bool) {
	values := i.raw[id]
	out := make([]T, 0, len(values))
	for _, v := range values {
		x, ok := v.Value.(T)
		if !ok {
			return nil, false
		}
		out = append(out, x)
	}
	return out, len(out) > 0
}
