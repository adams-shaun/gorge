package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMutateUnderCardKeywords pins CR 702.140e for the shape the divergence
// ticket measured: a creature card printing flying mutates UNDER a vanilla
// creature, and the resulting one-permanent pile has flying even though its
// top face prints none (chars.BaseKeywords walks the pile's under cards).
// When the pile leaves the battlefield it splits, and the flying goes home
// with the card that prints it.
func TestMutateUnderCardKeywords(t *testing.T) {
	t.Parallel()
	flier := card(t, "Name:Winged Lynx\nManaCost:2 U\nTypes:Creature Cat\nPT:2/2\n"+
		"K:Mutate:2 U\nK:Flying\nOracle:x\n")
	vanilla := card(t, "Name:Plains Bear\nManaCost:2 G\nTypes:Creature Bear\nPT:3/3\nOracle:x\n")
	e, cfg := tokenReplGame(t, 470, flier, vanilla)
	flierID := moveSeededCard(t, e, 0, flier, state.ZHand)
	bear := moveSeededCard(t, e, 0, vanilla, state.ZBattlefield)

	// Preconditions: flying is printed only on the under card's face, and the
	// lone bear does not derive it before the merge.
	if !flier.Faces[0].HasKeyword("Flying") {
		t.Fatalf("precondition: the Lynx does not print flying: %q", flier.Faces[0].Keywords)
	}
	if e.G.Obj(bear).Face().HasKeyword("Flying") {
		t.Fatalf("precondition: the top face prints flying: %q", e.G.Obj(bear).Face().Keywords)
	}
	if e.HasKeyword(bear, "Flying") {
		t.Fatalf("precondition: the bear already flies: %q", e.Derived(bear).Keywords)
	}

	addMana(t, e, 0, "UUU") // the mutate cost {2}{U}
	mutateCastOnto(t, e, mutatedCastOption(t, e, flierID), bear, false)
	mutateDrain(t, e, 40)

	pile := e.G.Obj(bear)
	if pile == nil || pile.Zone != state.ZBattlefield || pile.Face() == nil ||
		pile.Face().Name != "Plains Bear" || len(pile.MergedCards) != 1 ||
		pile.MergedFaceAt(0).Name != "Winged Lynx" {
		t.Fatalf("precondition: want the Lynx beneath the bear, got %+v", pile)
	}
	if !e.HasKeyword(bear, "Flying") {
		t.Fatalf("the pile lacks the under card's flying: derived %q", e.Derived(bear).Keywords)
	}

	// CR 702.140e: when a mutated permanent leaves the battlefield, each card
	// that represents it moves with it, so the pile splits and the flying
	// leaves with the card that prints it.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZHand})
	if o := e.G.Obj(bear); o.Zone != state.ZHand || len(o.MergedCards) != 0 {
		t.Fatalf("precondition: the pile split to hand: %+v", o)
	}
	if e.HasKeyword(bear, "Flying") {
		t.Fatalf("the top card kept flying after the pile left: %q", e.Derived(bear).Keywords)
	}
	replayCheck(t, e, cfg)
}
