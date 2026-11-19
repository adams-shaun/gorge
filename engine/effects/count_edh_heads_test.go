package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestEDHCountHeadsBImprintedManaCostIsTheSharedMaximum pins Psychic
// Battle's Count$ImprintedCardManaCost: Forge reads the FIRST imprinted
// card's mana value, and the card imprints every reveal tied for the highest,
// so with reveals of 3 and 3 the head reads 3 (never the sum 6), which a
// third seat's reveal of 5 then beats.
func TestEDHCountHeadsBImprintedManaCostIsTheSharedMaximum(t *testing.T) {
	h, c := fixtureHost(t)
	three := mkCard(t, "Name:Three\nManaCost:2 U\nTypes:Creature\nPT:1/1\nOracle:x\n")
	a := lzAdd(t, h, 0, state.ZLibrary, three)
	b := lzAdd(t, h, 1, state.ZLibrary, three)
	src := h.g.Obj(c.Source)
	src.Imprinted = []state.ObjID{a, b}
	if got := objectManaValue(h.g, h.g.Obj(a)); got != 3 {
		t.Fatalf("precondition: imprinted card mana value = %d, want 3", got)
	}
	if got := EvalCount(h, c, "ImprintedCardManaCost"); got != 3 {
		t.Errorf("ImprintedCardManaCost over a {3,3} tie = %d, want 3 (the shared maximum, not the sum 6)", got)
	}
	src.Imprinted = nil
	if got := EvalCount(h, c, "ImprintedCardManaCost"); got != 0 {
		t.Errorf("ImprintedCardManaCost with nothing imprinted = %d, want 0", got)
	}
}

// TestEDHCountHeadsBExactManaCostSkipsNoCost pins Embodiment of Agonies'
// Count$ExactManaCostGraveyard: a card whose cost is "no cost" (Living End,
// Ancestral Vision) is removed from the set of distinct mana costs, as
// Forge removes ManaCost.NO_COST.
func TestEDHCountHeadsBExactManaCostSkipsNoCost(t *testing.T) {
	h, c := fixtureHost(t)
	lzAdd(t, h, 0, state.ZGraveyard, mkCard(t, "Name:Bears\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n"))
	lzAdd(t, h, 0, state.ZGraveyard, mkCard(t, "Name:Bears Twin\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n"))
	lzAdd(t, h, 0, state.ZGraveyard, mkCard(t, "Name:Ox\nManaCost:2 W\nTypes:Creature\nPT:2/2\nOracle:x\n"))
	if got := EvalCount(h, c, "ExactManaCostGraveyard Card.nonLand+YouOwn"); got != 2 {
		t.Fatalf("precondition: two distinct costs among {1}{G},{1}{G},{2}{W} = %d, want 2", got)
	}
	lzAdd(t, h, 0, state.ZGraveyard, mkCard(t, "Name:Living End Like\nManaCost:no cost\nTypes:Sorcery\nOracle:x\n"))
	if got := EvalCount(h, c, "ExactManaCostGraveyard Card.nonLand+YouOwn"); got != 2 {
		t.Errorf("a 'no cost' card changed the distinct-cost count to %d, want 2", got)
	}
}

// TestEDHCountHeadsBChromaSkipsPhasedOut pins Springjack Shepherd's
// Count$Chroma: a phased-out permanent is treated as though it does not
// exist (CR 702.26b), so its mana symbols do not count.
func TestEDHCountHeadsBChromaSkipsPhasedOut(t *testing.T) {
	h, c := fixtureHost(t)
	id := lzAdd(t, h, 0, state.ZBattlefield, mkCard(t, "Name:Whiteling\nManaCost:W W\nTypes:Creature\nPT:1/1\nOracle:x\n"))
	if got := EvalCount(h, c, "Chroma.White"); got != 2 {
		t.Fatalf("precondition: chroma of a {W}{W} permanent = %d, want 2", got)
	}
	h.g.Obj(id).PhasedOut = true
	if got := EvalCount(h, c, "Chroma.White"); got != 0 {
		t.Errorf("chroma with the only permanent phased out = %d, want 0", got)
	}
}
