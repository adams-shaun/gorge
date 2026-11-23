package adapter

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAskSourceCarriesTheElectionSubject pins the served source of a pending
// as-unblocked election (dirac.ChAsUnblocked): gorge serves the election
// KChoose with Source set to the attacker (engine/rules combat.go's
// as-unblocked ask), and the election's options already name it — like a
// division's, the canon translation shares the shape (canon/translate.go
// ChDivide, ChAsUnblocked) — so askSource must take the option's object
// instead of falling back to the kernel features. At a combat damage step
// those provably name nothing (FeatEnterSource's no-gen-ask arm,
// FeatSource's non-build non-resolve arm): a missing case loses the wire
// decision's subject (Source 0), and oracle's decisionDiffs "source" reddens
// on the first evidence pool that carries a carrier.
func TestAskSourceCarriesTheElectionSubject(t *testing.T) {
	const attacker state.ObjID = 12
	g := &Game{d: dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChAsUnblocked, N: 2}}
	wire := &decision.Decision{
		Kind:    decision.KChoose,
		Options: []decision.Option{{Kind: "asunblocked", Obj: attacker}, {Kind: "asunblocked", Obj: attacker}},
	}
	if got := g.askSource(wire); got != attacker {
		t.Fatalf("election askSource %d, want the attacker %d", got, attacker)
	}
	// Control: the same-shaped options under a Sub the case does not cover
	// still serve 0, because on this kernel no feature names an object — the
	// precondition that makes the election case load-bearing rather than a
	// coincidence of the options.
	g2 := &Game{d: dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChGenPileChoose, N: 1}}
	wire2 := &decision.Decision{Kind: decision.KChoose, Options: []decision.Option{{Kind: "choose", Obj: attacker}}}
	if got := g2.askSource(wire2); got != 0 {
		t.Fatalf("non-election askSource %d, want 0 (no kernel feature names an object here)", got)
	}
}
