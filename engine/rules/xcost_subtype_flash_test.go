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

// scrutinyCastWithX begins an off-turn cast of Silver Scrutiny and answers
// the {X} announcement with x, then reports the card's zone.
func scrutinyCastWithX(t *testing.T, x int) state.Zone {
	t.Helper()
	e, spell := offTurnFlashEngine(t, "Silver Scrutiny", 4, 2)
	if !hasCastOption(e.legalActions(0), spell) {
		t.Fatal("Silver Scrutiny (X <= 3 grants flash) was not offered off-turn")
	}
	e.beginCast(0, decision.Option{Kind: "cast", Obj: spell})
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("want the X announcement, got %+v", d)
	}
	idx := -1
	for i, o := range d.Options {
		if o.Kind == "x" && o.Amount == x {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatalf("precondition: X=%d not offered: %+v", x, d.Options)
	}
	submitChoices(t, e, idx)
	return e.G.Obj(spell).Zone
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

	t.Run("XCostLE3 X=3 casts", func(t *testing.T) {
		if z := scrutinyCastWithX(t, 3); z != state.ZStack {
			t.Fatalf("X=3 off-turn Silver Scrutiny ended in %s, want stack", z)
		}
	})
	t.Run("XCostLE3 X=4 withheld", func(t *testing.T) {
		if z := scrutinyCastWithX(t, 4); z != state.ZHand {
			t.Fatalf("X=4 off-turn Silver Scrutiny ended in %s, want hand (CR 601.2e reversal)", z)
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
