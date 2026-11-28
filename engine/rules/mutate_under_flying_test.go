package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMutatedPermanentHasUnderCardKeywords (CR 702.140e): Everquill Phoenix,
// which prints flying, mutated under a creature without it gives the pile
// flying; the pile leaving the battlefield takes the flying with it.
func TestMutatedPermanentHasUnderCardKeywords(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	e, cfg := tokenReplGame(t, 461, phoenix)
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	bear := putToken(t, e, 0, mutateBearSrc, state.ZBattlefield)
	if !phoenix.Faces[0].HasKeyword("Flying") {
		t.Fatalf("precondition: the Phoenix does not print flying: %q", phoenix.Faces[0].Keywords)
	}
	if e.HasKeyword(bear, "Flying") {
		t.Fatalf("precondition: the bear already flies: %q", e.Derived(bear).Keywords)
	}
	addMana(t, e, 0, "RRRR") // the mutate cost {3}{R}

	mutateCastOnto(t, e, mutatedCastOption(t, e, phoenixID), bear, false)
	mutateDrain(t, e, 40)

	pile := e.G.Obj(bear)
	if pile == nil || pile.Zone != state.ZBattlefield || pile.Face() == nil || pile.Face().Name != "Mutate Bear" ||
		len(pile.MergedCards) != 1 || pile.MergedFaceAt(0).Name != "Everquill Phoenix" {
		t.Fatalf("precondition: want the Phoenix beneath the bear, got %+v", pile)
	}
	if !e.HasKeyword(bear, "Flying") {
		t.Fatalf("the pile lacks the under card's flying: derived %q", e.Derived(bear).Keywords)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZHand})
	if o := e.G.Obj(bear); o.Zone != state.ZHand || len(o.MergedCards) != 0 {
		t.Fatalf("precondition: the pile split to hand: %+v", o)
	}
	if e.HasKeyword(bear, "Flying") {
		t.Fatalf("the top card kept flying after the pile left: %q", e.Derived(bear).Keywords)
	}
	replayCheck(t, e, cfg)
}
