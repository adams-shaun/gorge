package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFilterFatalCopiesIgnoresEqualCopies pins the rule's word "strictly":
// two identical creatures are neither worse than the other, so neither is
// demoted and the ranking still prefers one of them over an unrelated
// smaller creature. A non-strict comparison demoted both 4/4s and picked
// the 3/3 instead.
func TestTargetRuleFilterFatalCopiesIgnoresEqualCopies(t *testing.T) {
	tsig := TypeSigOf([]string{"Creature", "Goblin"})
	cs := map[state.ObjID]Card{
		10: {PrintedName: "Goblin A", Creature: true, Power: 4, Toughness: 4, TypeSig: tsig},
		11: {PrintedName: "Goblin B", Creature: true, Power: 4, Toughness: 4, TypeSig: tsig},
		12: {PrintedName: "Elf", Creature: true, Power: 3, Toughness: 3, TypeSig: TypeSigOf([]string{"Creature", "Elf"})},
	}
	b := ruleBoard(cs, []state.ObjID{10, 11, 12}, nil)
	d := removalDecision([]tgt{
		{kind: "permanent", obj: 10, pl: 1},
		{kind: "permanent", obj: 11, pl: 1},
		{kind: "permanent", obj: 12, pl: 1},
	})

	// Precondition: the two copies really are equal on every priced fact, so
	// the only difference the rule can see is equality itself; and the 4/4 is
	// strictly better than the 3/3, so the comparison can distinguish them.
	a := Creature{Power: 4, Toughness: 4}
	equal := Creature{Power: 4, Toughness: 4}
	if strictlyBetter(a, equal) || strictlyBetter(equal, a) {
		t.Fatal("precondition: identical creatures must not be strictly better than each other")
	}
	if !strictlyBetter(a, Creature{Power: 3, Toughness: 3}) {
		t.Fatal("precondition: a 4/4 should be strictly better than a 3/3")
	}

	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("default choices = %v, want [0] (a 4/4)", got)
	}
	b.Targets = TargetRules{FilterFatalCopies: true}
	if b.worseCopy(10, 0) {
		t.Fatal("worseCopy(goblin A) = true, want false beside an identical copy")
	}
	if b.worseCopy(11, 0) {
		t.Fatal("worseCopy(goblin B) = true, want false beside an identical copy")
	}
	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("armed choices = %v, want [0] (a 4/4, not the unrelated 3/3)", got)
	}
}

// TestProminentColourIgnoresNonCreatures pins "a creature of the dominant
// colour": a dominant-colour artifact earns nothing, and a dominant-colour
// land earns only its land bonus (20), never the prominent +2 on top.
func TestTargetRuleProminentColourIgnoresNonCreatures(t *testing.T) {
	cs := map[state.ObjID]Card{
		20: {PrintedName: "Red Relic", Colour: cards.ColourMaskRed},
		21: {PrintedName: "Mountain", Land: true, Colour: cards.ColourMaskRed},
	}
	public := map[state.ObjID]PublicCard{
		20: {Controller: 1, Colour: cards.ColourMaskRed},
		21: {Controller: 1, Colour: cards.ColourMaskRed},
	}
	b := ruleBoard(cs, nil, public)

	// Precondition: red is the target controller's dominant colour, and the
	// artifact is where the rule looks (a Card row) but is not a creature.
	if got := dominantColour(b.publicColourMix(1)); got < 0 || cards.ColourMask(1<<got) != cards.ColourMaskRed {
		t.Fatalf("precondition: dominant colour = %d, want red", got)
	}
	if c, _ := b.Cards.Lookup(20); c.Creature {
		t.Fatal("precondition: the artifact must be a non-creature")
	}

	b.Targets = TargetRules{ProminentColour: true}
	if got := b.a2RemovalBonus(decision.Option{Obj: 20, Controller: 1}); got != 0 {
		t.Fatalf("non-creature prominent bonus = %d, want 0", got)
	}

	b.Targets = TargetRules{LandPriority: true, ProminentColour: true}
	if got := b.a2RemovalBonus(decision.Option{Obj: 21, Controller: 1}); got != landDominantBonus {
		t.Fatalf("dominant-colour land bonus = %d, want %d (land only)", got, landDominantBonus)
	}
}
