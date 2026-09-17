package searchprobe

import "fmt"

// Failure distinguishes input contradictions and unsupported observation shapes
// from internal invariants. Prefix rejection/budget exhaustion are counters, not
// proofs that the supplied history is impossible.
type Failure struct{ Kind, Detail string }

func (f *Failure) Error() string { return f.Kind + ": " + f.Detail }
func fail(kind, format string, args ...any) error {
	return &Failure{Kind: kind, Detail: fmt.Sprintf(format, args...)}
}
