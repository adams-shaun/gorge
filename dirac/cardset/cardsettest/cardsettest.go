// Package cardsettest is a test's door to card sets. Only a tree with no corpus at
// all skips, matching gorge's testutil.CorpusRegistry; a corpus without this
// binary's tables fails, because a stale tree reading as green hides that nothing ran.
package cardsettest

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Set returns the set named name with every lowered card playable: a fixture tests
// what a card does, whether or not it is supported yet. With no corpus located it
// skips the test; on any other error (no tables for this binary, a damaged table, an
// unknown pool) it fails the test, naming the command when that is what is missing.
func Set(tb testing.TB, name string) cardset.Set {
	tb.Helper()
	s, err := cardset.RawByName(name)
	if errors.Is(err, cardset.ErrNoCorpus) {
		tb.Skipf("set %s: %v", name, err)
	}
	if err != nil {
		tb.Fatalf("set %s: %v", name, err)
	}
	return s
}

// Require is Set's check, for a test that reaches a set indirectly.
func Require(tb testing.TB, name string) {
	tb.Helper()
	Set(tb, name)
}

// Synth is Set for a synthetic evidence pool, with the same rule: no corpus skips,
// anything else fails.
func Synth(tb testing.TB, name string) cardset.Set {
	tb.Helper()
	s, err := cardset.SynthByName(name)
	if errors.Is(err, cardset.ErrNoCorpus) {
		tb.Skipf("synthetic pool %s: %v", name, err)
	}
	if err != nil {
		tb.Fatalf("synthetic pool %s: %v", name, err)
	}
	return s
}

// SynthNames is the synthetic pools, with Synth's rule.
func SynthNames(tb testing.TB) []string {
	tb.Helper()
	names, err := cardset.SynthNames()
	if errors.Is(err, cardset.ErrNoCorpus) {
		tb.Skipf("synthetic pools: %v", err)
	}
	if err != nil {
		tb.Fatalf("synthetic pools: %v", err)
	}
	return names
}
