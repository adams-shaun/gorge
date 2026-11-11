package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBeginTurnShapeGateRegistry: repl:BeginTurn is registered for the
// implemented ExtraTurn$ True | Skip$ True shape only; Time Vault's
// normal-turn optional skip stays unsupported as repl:BeginTurn.OtherShape.
func TestBeginTurnShapeGateRegistry(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Trouble in Pairs", "Stranglehold", "Ugin's Nexus", "Gerrard's Hourglass Pendant"} {
		if got := reg.Unsupported(lookup(t, reg, name), effects.Supported()); len(got) != 0 {
			t.Errorf("%s unsupported: %v", name, got)
		}
	}
	got := reg.Unsupported(lookup(t, reg, "Time Vault"), effects.Supported())
	if !slices.Contains(got, "repl:BeginTurn.OtherShape") || slices.Contains(got, "repl:BeginTurn") {
		t.Fatalf("Time Vault reasons = %v, want repl:BeginTurn.OtherShape and not the bare name", got)
	}
}
