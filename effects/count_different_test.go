package effects

// Task diffcount1: the Different* distinct-set property family of a
// Count$Valid<zone> <spec>$<Property> body counts DISTINCT VALUES among the
// matching cards, not the cards themselves. Before this, the property was
// unrecognised and the whole token became the spec (so it matched nothing and
// read 0). These pin the evaluator directly; the end-to-end Eris carrier is
// pinned in rules/different_card_manacost_test.go.

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// diffObject places a fresh card from src into p's zone and returns it.
func diffObject(t *testing.T, g *state.Game, p state.PlayerID, zone state.Zone, src string) state.ObjID {
	t.Helper()
	o := g.AddObject(mkCard(t, src), p)
	o.Zone = zone
	g.SetZone(zone, p, append(g.Zone(zone, p), o.ID))
	return o.ID
}

const (
	diffInstant1Src   = "Name:Trick One\nManaCost:U\nTypes:Instant\nOracle:x\n"
	diffInstant1bSrc  = "Name:Trick Two\nManaCost:1 U\nTypes:Instant\nOracle:x\n"
	diffSorcery3Src   = "Name:Rite\nManaCost:2 R\nTypes:Sorcery\nOracle:x\n"
	diffCreature2Src  = "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	diffCreature2bSrc = "Name:Elf\nManaCost:1 G\nTypes:Creature Elf\nPT:2/2\nOracle:x\n"
	diffCreature5Src  = "Name:Giant\nManaCost:3 R\nTypes:Creature Giant\nPT:5/5\nOracle:x\n"
)

// TestEvalCountDifferentCardManaCostDedupsDistinctValues pins the core read:
// three instant/sorcery cards in your graveyard whose mana values are 1, 2 and
// 3 count as THREE, and a duplicate mana value does not add a fourth.
func TestEvalCountDifferentCardManaCostDedupsDistinctValues(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZGraveyard, diffInstant1Src)
	diffObject(t, g, 0, state.ZGraveyard, diffInstant1bSrc)
	diffObject(t, g, 0, state.ZGraveyard, diffSorcery3Src)
	diffObject(t, g, 0, state.ZGraveyard, diffCreature2Src) // a creature: not matched
	// PRECONDITION: the four cards really are in your graveyard; without them
	// a zero would pass the assertion below for the wrong reason.
	if got := len(g.Zone(state.ZGraveyard, 0)); got != 4 {
		t.Fatalf("graveyard holds %d cards, want 4", got)
	}
	const body = "Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn$DifferentCardManaCost"
	if got := EvalCount(h, c, body); got != 3 {
		t.Fatalf("%s = %d, want 3 distinct mana values", body, got)
	}
	// A duplicate mana value (another {1} instant) does not move the count --
	// this is what distinguishes the head from a plain card count.
	diffObject(t, g, 0, state.ZGraveyard, diffInstant1bSrc)
	if got := EvalCount(h, c, body); got != 3 {
		t.Fatalf("%s after a duplicate MV = %d, want still 3", body, got)
	}
}

// TestEvalCountDifferentCardManaCostTimesSuffix multiplies the distinct count
// by the /Times.N op suffix -- Eris, Roar of the Storm's shape, where each
// distinct mana value is {2} off.
func TestEvalCountDifferentCardManaCostTimesSuffix(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZGraveyard, diffInstant1Src)
	diffObject(t, g, 0, state.ZGraveyard, diffSorcery3Src)
	if got := len(g.Zone(state.ZGraveyard, 0)); got != 2 {
		t.Fatalf("graveyard holds %d cards, want 2", got)
	}
	const body = "Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn$DifferentCardManaCost/Times.2"
	if got := EvalCount(h, c, body); got != 4 {
		t.Fatalf("%s = %d, want 4 (two distinct values, {2} each)", body, got)
	}
}

// TestEvalCountDifferentCardPowerUsesDerivedPower pins the sibling: distinct
// DERIVED powers, so two 2/2s count once and a 5/5 adds a second.
func TestEvalCountDifferentCardPowerUsesDerivedPower(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZBattlefield, diffCreature2Src)
	diffObject(t, g, 0, state.ZBattlefield, diffCreature2bSrc)
	diffObject(t, g, 0, state.ZBattlefield, diffCreature5Src)
	if got := len(g.Zone(state.ZBattlefield, 0)); got != 3 {
		t.Fatalf("battlefield holds %d creatures, want 3", got)
	}
	const body = "Count$Valid Creature.YouCtrl$DifferentCardPower"
	if got := EvalCount(h, c, body); got != 2 {
		t.Fatalf("%s = %d, want 2 distinct powers (two 2/2s, one 5/5)", body, got)
	}
}

// TestEvalCountDifferentCardNamesDedupsNames pins the name spelling, and that
// an unmodelled Different spelling still fails closed (zero), not open.
func TestEvalCountDifferentCardNamesDedupsNames(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0}
	diffObject(t, g, 0, state.ZBattlefield, diffCreature2Src)
	diffObject(t, g, 0, state.ZBattlefield, diffCreature2bSrc)
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl$DifferentCardNames"); got != 2 {
		t.Fatalf("DifferentCardNames = %d, want 2 (two differently named creatures)", got)
	}
	// An unmodelled Different spelling keeps the historical fail-closed read.
	if got := EvalCount(h, c, "Count$Valid Creature.YouCtrl$DifferentNotARealProperty"); got != 0 {
		t.Fatalf("unknown Different property = %d, want 0 (fail closed)", got)
	}
}

// TestDifferentPropertyValueNilFaceContributesNothing pins the defensive
// guard a review round added: a remembered/targeted shell object with no
// card face (Face() nil -- a Card==nil or out-of-range FaceIdx object,
// state/object.go) contributes nothing to a distinct-value set instead of
// panicking the match. Called directly because no public EvalCount path
// currently routes a matching-but-nil-face object into the fold, so this
// is the one place the guard is provably reachable.
func TestDifferentPropertyValueNilFaceContributesNothing(t *testing.T) {
	if v, ok := differentPropertyValue(nil, &state.Object{}, diffManaCost); ok {
		t.Fatalf("nil-face object contributed value %d, want ok=false", v)
	}
}

// TestEvalCountDifferentCardManaCostRefProperty pins the Remembered$ spelling
// (Azor's Gateway / Atemsis): the same distinct-value fold over a reference's
// objects.
func TestEvalCountDifferentCardManaCostRefProperty(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	h := &fakeHost{g: g}
	a := diffObject(t, g, 0, state.ZBattlefield, diffCreature2Src)
	b := diffObject(t, g, 0, state.ZBattlefield, diffCreature5Src)
	diffObject(t, g, 0, state.ZBattlefield, diffCreature2bSrc)
	c := &Ctx{Controller: 0}
	c.Remembered = []state.Target{{Obj: a}, {Obj: b}}
	if got := EvalCount(h, c, "Remembered$DifferentCardManaCost"); got != 2 {
		t.Fatalf("Remembered$DifferentCardManaCost = %d, want 2 (MV 2 and 5)", got)
	}
	// PRECONDITION: the two remembered objects are distinct and on the board.
	if a == b {
		t.Fatal("the two remembered objects must be distinct")
	}
}
