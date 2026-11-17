package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

type fakeWitnessTargets struct {
	available  bool
	candidates []targetCandidate
}

func (f fakeWitnessTargets) legalTargetCandidates(state.PlayerID, state.ObjID, state.ObjID, *cards.SA) []targetCandidate {
	return f.candidates
}

func (f fakeWitnessTargets) castTargetsAvailable(state.PlayerID, state.ObjID, *cards.SA) bool {
	return f.available
}

// TestWitnessAnnouncesBeforePaying: a cast the witness's sacrifice leaves with
// no target is still payable when a candidate target other than the sacrificed
// permanent was announceable first (CR 601.2c-h): Confound over a spell that
// targets the Spawn the witness sacrifices. It stays refused when the
// sacrificed permanent was every candidate, or when the cast still has its
// targets at the end.
func TestWitnessAnnouncesBeforePaying(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 9416, "Name:Witness Counter\nManaCost:1 U\nTypes:Instant\nA:SP$ Counter | TargetType$ Spell | ValidTgts$ Card | TgtPrompt$ Select target spell\nOracle:x\n")
	opt := decision.Option{Kind: "cast", Obj: spell}
	spawn, other := state.ObjID(901), state.ObjID(902)
	plan := decision.PaymentPlan{Activations: []decision.PaymentActivation{{Source: spawn, Consequence: &decision.PaymentConsequence{Sacrifice: true}}}}
	root := func(cands ...targetCandidate) fakeWitnessTargets { return fakeWitnessTargets{true, cands} }
	gone := fakeWitnessTargets{}
	for _, tc := range []struct {
		name      string
		root, end fakeWitnessTargets
		want      bool
	}{
		{"candidate survives the sacrifice", root(targetCandidate{kind: "obj", obj: other}), gone, true},
		{"the sacrificed permanent was the only candidate", root(targetCandidate{kind: "obj", obj: spawn}), gone, false},
		{"targets remain at the end", root(targetCandidate{kind: "obj", obj: other}), fakeWitnessTargets{available: true}, false},
		{"not announceable at the root", fakeWitnessTargets{candidates: []targetCandidate{{kind: "obj", obj: other}}}, gone, false},
	} {
		if got := witnessAnnouncesBeforePaying(tc.root, tc.end, e.G, 0, opt, plan); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
	// the precondition: the fixture's spell carries a spell ability the helper reads.
	if o := e.G.Obj(spell); o == nil || o.Face() == nil || o.Face().SpellAbility() == nil {
		t.Fatal("the fixture spell has no spell ability")
	}
}
