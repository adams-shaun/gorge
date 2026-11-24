package effects

// count_devotion_prototype_test.go pins CR 718.3b for Count$Devotion: a
// prototyped permanent has only its prototype characteristics, its mana cost
// included, so devotion counts the symbols in the prototype cost, not the
// printed one. dirac's devotion already reads the prototype face's cost; this
// is the gorge half of the lockstep fix.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDevotionCountsPrototypeCostNotPrintedCost is the DEVOTION leaf: a
// prototyped Cradle Clearcutter ({6} printed, {1}{G} prototype) contributes
// one green symbol, and the same card cast for its printed cost contributes
// none.
func TestDevotionCountsPrototypeCostNotPrintedCost(t *testing.T) {
	const src = "Name:Cradle Clearcutter\nManaCost:6\nTypes:Artifact Creature Construct\nPT:2/2\nK:Prototype:1 G:3:3\nOracle:x\n"
	g := state.NewGame([]string{"proto", "other"})
	card := cardOf(t, src)

	proto := g.AddObject(card, 0)
	proto.Zone = state.ZBattlefield
	proto.CastFlags |= state.FlagPrototype
	plain := g.AddObject(card, 0)
	plain.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), proto.ID, plain.ID))

	// Preconditions: both permanents are on seat 0's battlefield, the
	// prototyped one is marked, and the printed and prototype costs really
	// differ in green symbols -- a vacuous setup would fail here.
	if n := len(g.Zone(state.ZBattlefield, 0)); n != 2 {
		t.Fatalf("fixture broken: seat 0 battlefield holds %d objects, want 2", n)
	}
	if proto.CastFlags&state.FlagPrototype == 0 || plain.CastFlags&state.FlagPrototype != 0 {
		t.Fatalf("fixture broken: prototyped=%v plain=%v", proto.CastFlags, plain.CastFlags)
	}
	pf := proto.Face()
	if pf == nil || pf.ManaCost != "6" {
		t.Fatalf("fixture broken: prototyped face cost = %+v, want printed {6}", pf)
	}
	if got := cards.ManaCostChroma(pf.ManaCost, cards.ColourGreen); got != 0 {
		t.Fatalf("fixture broken: printed cost {6} has %d green pips, want 0", got)
	}
	cost, _, _, ok := proto.PrototypeCharacteristics()
	if !ok || cost != "1 G" {
		t.Fatalf("fixture broken: prototype characteristics = (%q, %v), want (\"1 G\", true)", cost, ok)
	}
	if got := cards.ManaCostChroma(cost, cards.ColourGreen); got != 1 {
		t.Fatalf("fixture broken: prototype cost %q has %d green pips, want 1", cost, got)
	}
	if _, _, _, ok := plain.PrototypeCharacteristics(); ok {
		t.Fatal("fixture broken: the plain permanent reports prototype characteristics")
	}

	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Source: proto.ID}
	if got, ok := EvalCountOK(h, c, "Count$Devotion.Green"); !ok || got != 1 {
		t.Fatalf("Count$Devotion.Green = (%d, %v), want (1, true): {1}{G} prototype counts one green pip, printed {6} counts none (CR 718.3b)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$Devotion.Blue"); !ok || got != 0 {
		t.Fatalf("Count$Devotion.Blue = (%d, %v), want (0, true)", got, ok)
	}
}
