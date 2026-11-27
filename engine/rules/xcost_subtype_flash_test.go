package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// xcostSubtypeCWF checks the corpus carrier's CastWithFlash static still
// carries the ValidSA$ tail this test pins.
func xcostSubtypeCWF(t *testing.T, c *cards.Card, validSA string) {
	t.Helper()
	for _, f := range c.Faces {
		for _, s := range f.Statics {
			if s.Mode == "CastWithFlash" && s.ParamStr(cards.PKValidSA) == validSA {
				return
			}
		}
	}
	t.Fatalf("precondition: %s has no CastWithFlash with ValidSA$ %s", c.Faces[0].Name, validSA)
}

// scrutinyXAsk begins a cast of Silver Scrutiny with {4}{U}{U} floating
// (X up to 4 payable), off-turn when offTurn, and returns the engine, the
// card and the X values the announcement offers.
func scrutinyXAsk(t *testing.T, offTurn bool) (*Engine, state.ObjID, []int) {
	t.Helper()
	e, spell := offTurnFlashEngine(t, "Silver Scrutiny", 4, 2)
	if !offTurn {
		e.G.Active = 0
		if !e.sorcerySpeed(0) {
			t.Fatal("precondition: seat 0 has sorcery timing on its own turn")
		}
	} else if e.sorcerySpeed(0) {
		t.Fatal("precondition: seat 0 has no sorcery timing off-turn")
	}
	if !hasCastOption(e.legalActions(0), spell) {
		t.Fatalf("Silver Scrutiny not offered (offTurn=%v)", offTurn)
	}
	e.beginCast(0, decision.Option{Kind: "cast", Obj: spell})
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the X announcement, got %+v", d)
	}
	var xs []int
	for _, o := range d.Options {
		if o.Kind == "x" {
			xs = append(xs, o.Amount)
		}
	}
	return e, spell, xs
}

// TestXCostSubtypeValidSAFlash pins the two spell-side ValidSA$ tails a
// CastWithFlash permission reads: Silver Scrutiny's self grant
// `Spell.XCostLE3` (flash only when the announced X is 3 or less, the CR
// 601.2e recheck reversing X=4) and Blur of Heroism's `Spell.Hero` (flash for
// Hero spells only).
func TestXCostSubtypeValidSAFlash(t *testing.T) {
	t.Parallel()
	xcostSubtypeCWF(t, corpusAlternativeCard(t, "Silver Scrutiny"), "Spell.XCostLE3")
	xcostSubtypeCWF(t, corpusAlternativeCard(t, "Blur of Heroism"), "Spell.Hero")

	t.Run("XCostLE3 sorcery timing offers X=4", func(t *testing.T) {
		_, _, xs := scrutinyXAsk(t, false)
		if len(xs) == 0 || xs[len(xs)-1] != 4 {
			t.Fatalf("sorcery-timed X menu = %v, want up to 4 (the grant does not bound it)", xs)
		}
	})
	t.Run("XCostLE3 off-turn caps X at 3 and casts", func(t *testing.T) {
		e, spell, xs := scrutinyXAsk(t, true)
		if len(xs) == 0 || xs[len(xs)-1] != 3 {
			t.Fatalf("off-turn X menu = %v, want capped at 3", xs)
		}
		answerX(t, e, 3)
		if z := e.G.Obj(spell).Zone; z != state.ZStack {
			t.Fatalf("X=3 off-turn Silver Scrutiny ended in %s, want stack", z)
		}
	})
	t.Run("XCostLE3 recheck rejects X=4", func(t *testing.T) {
		e, spell, _ := scrutinyXAsk(t, true)
		f := e.G.Obj(spell).Face()
		if e.cast == nil || e.cast.card != spell || !e.cast.xDone || !e.cast.offSorcery {
			t.Fatal("precondition: the off-turn X announcement is pending")
		}
		e.cast.x = 3
		if !e.flashGrantCoversTargets(0, spell, f, nil) {
			t.Fatal("CR 601.2e: X=3 must keep the XCostLE3 permission")
		}
		e.cast.x = 4
		if e.flashGrantCoversTargets(0, spell, f, nil) {
			t.Fatal("CR 601.2e: X=4 must lose the XCostLE3 permission")
		}
	})

	t.Run("Hero", func(t *testing.T) {
		hero := card(t, "Name:Test Hero\nManaCost:1 W\nTypes:Creature Human Hero\nPT:2/2\nOracle:x\n")
		plain := card(t, "Name:Test Soldier\nManaCost:1 W\nTypes:Creature Human Soldier\nPT:2/2\nOracle:x\n")
		e := handEngine(t, hero, plain)
		e.G.Active = 1
		e.G.Players[0].Pool[state.MC] = 1
		e.G.Players[0].Pool[state.MW] = 1
		hand := e.G.Zone(state.ZHand, 0)
		heroID, plainID := hand[0], hand[1]
		if hasCastOption(e.legalActions(0), heroID) || hasCastOption(e.legalActions(0), plainID) {
			t.Fatal("precondition: off-turn creatures offered with no Blur of Heroism")
		}
		blur := e.G.AddObject(corpusAlternativeCard(t, "Blur of Heroism"), 0)
		blur.Zone = state.ZBattlefield
		e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), blur.ID))
		opts := e.legalActions(0)
		if !hasCastOption(opts, heroID) {
			t.Fatal("Blur of Heroism did not grant flash to a Hero spell")
		}
		if hasCastOption(opts, plainID) {
			t.Fatal("Blur of Heroism granted flash to a non-Hero spell")
		}
	})
}
