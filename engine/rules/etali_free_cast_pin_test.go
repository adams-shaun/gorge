package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestEtaliFreeCastElectionEmptyWithOnlyAnInfeasibleSpellExiled pins the
// lockstep pair 23256 divergence triage at decision 695 (kernel-land): Etali,
// Primal Storm's attack trigger exiles the top of each library and then may
// cast any number of the remembered spells without paying their mana costs.
// With an opposing side that controls no creature, the only remembered
// non-land spell is Felling Blow, whose mandatory second target ("target
// creature an opponent controls", CR 601.2c) cannot be chosen, so the
// Optional$ election is empty: the trigger resolves without posing a cast ask
// and Felling Blow stays in exile. The dirac kernel's OpCastFree
// (kernel/interp_gen.go) keeps every remembered non-land card in its free
// list and poses the raGenCastFree ask instead; this test is the gorge-side
// pin that the kernel round must meet, not a gorge change.
func TestEtaliFreeCastElectionEmptyWithOnlyAnInfeasibleSpellExiled(t *testing.T) {
	t.Parallel()
	run := runInlineOracle(t, `{
	  "name": "etali-free-cast-election-empty-infeasible-spell",
	  "setup": {"p0": {"battlefield": ["Etali, Primal Storm"], "library_top": ["Felling Blow", "Mountain"]},
	            "p1": {"battlefield": [], "library_top": ["Forest", "Plains"]}},
	  "steps": [
	    {"op": "attack", "seat": 0, "attackers": ["p0:Etali, Primal Storm"], "defender": "p1"},
	    {"op": "resolve"}
	  ],
	  "expect": [{"card": "p0:Felling Blow", "zone": "exile"},
	             {"card": "p1:Forest", "zone": "exile"}]
	}`)
	blow := run.refs["p0:Felling Blow"]
	if blow == 0 {
		t.Fatal("setup: p0:Felling Blow was never bound (exile precondition missing)")
	}
	attacked := false
	for _, od := range run.decisions {
		if od.GorgeKind == "attackers" {
			attacked = true
		}
		if od.GorgeKind == "modes" {
			t.Fatalf("a modes ask was posed although the free-cast election had no candidate: %+v", od)
		}
	}
	if !attacked {
		t.Fatal("precondition: the run recorded no attackers decision (the combat did not happen)")
	}
	for _, ev := range run.e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == blow {
			t.Fatalf("Felling Blow was offered and cast with no legal second target (event %d)", ev.Seq)
		}
	}
}
