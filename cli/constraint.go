package cli

import "fmt"

type ConstraintKind string

const (
	Conflicts      ConstraintKind = "conflicts"
	Requires       ConstraintKind = "requires"
	ExactlyOne     ConstraintKind = "exactly_one"
	AtLeastOne     ConstraintKind = "at_least_one"
	AllOrNone      ConstraintKind = "all_or_none"
	MinOccurrences ConstraintKind = "min_occurrences"
	MaxOccurrences ConstraintKind = "max_occurrences"
	ValuePredicate ConstraintKind = "value_predicate"
)

// ConstraintValues exposes effective, resolved values to value-dependent
// constraints without exposing parser/compiler internals.
type ConstraintValues struct {
	values map[string][]any
}

func (v ConstraintValues) Present(id string) bool { return len(v.values[id]) > 0 }
func (v ConstraintValues) Values(id string) []any {
	return append([]any(nil), v.values[id]...)
}
func ConstraintValueAs[T any](v ConstraintValues, id string) (T, bool) {
	var zero T
	xs := v.values[id]
	if len(xs) == 0 {
		return zero, false
	}
	x, ok := xs[len(xs)-1].(T)
	return x, ok
}

// Constraint describes declarative presence/count rules or a stable, named
// value predicate. PredicateID is part of schema/contract output so callers can
// reason about the semantic boundary even though the function itself is not
// serialized.
type Constraint struct {
	Kind        ConstraintKind
	IDs         []string
	Message     string
	Min         int
	Max         int
	PredicateID string
	Validate    func(ConstraintValues) error
}

func constraintMessage(kind ConstraintKind, ids []string, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return fmt.Sprintf("constraint %s failed for %v", kind, ids)
}
