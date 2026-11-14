package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A convoke + kicker creature. The kicked cast's total is the credit-folded
// base plus the kicker raise (CR 601.2b, 601.2f), so the menu's gate prices
// the same creature taps the plain cast's gate does.
const cokKickerSrc = "Name:Creditor\nManaCost:5 G\nTypes:Creature Elf\nPT:3/3\nK:Convoke\nK:Kicker:R\nOracle:x\n"

// A colourless convoke fodder creature: its tap pays one generic of the
// greedy convoke credit (the card's pips are {G}; the body has no colour).
const cokBodySrc = "Name:Creditor Body\nManaCost:1\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"

// TestConvokeCreditsKickedCastOffer (CR 601.2b, 601.2f): with {4} plus {R}
// floating and one untapped creature on the battlefield the menu offers both
// casts -- the plain one ({4}{G} less the credit) and the kicked one
// (castOfferBase plus the {R} raise). Pricing the kicked gate on RawBaseCost
// ({5}{G} plus {R}) withheld it although the credit made it payable. At one
// mana less the kicked cast stays withheld: only the raise is missing, so the
// assertion tracks the gate, not the plain offer.
func TestConvokeCreditsKickedCastOffer(t *testing.T) {
	t.Parallel()
	e, cfg, spell := newFixtureDeck(t, 6308, cokKickerSrc, cokBodySrc)
	body := moveSeeded(t, e, 0, cokBodySrc, state.ZBattlefield)
	if o := e.G.Obj(body); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: the fodder is not an untapped battlefield permanent: %+v", o)
	}
	if o := e.G.Obj(spell); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: the convoke-kicker card is not in hand: %+v", o)
	}
	addMana(t, e, 0, "GGGGG")
	opts := castOptions(t, e)
	kicked := -1
	plain := false
	for _, o := range opts {
		if o.Obj != spell {
			continue
		}
		if o.Mode == "kicked" {
			kicked = o.Index
		} else {
			plain = true
		}
	}
	if !plain {
		t.Fatalf("precondition: the plain cast is not offered over the {4} pool: %+v", opts)
	}
	if kicked >= 0 {
		t.Fatalf("precondition: the kicked cast is offered although its {R} raise is unfunded: %+v", opts)
	}
	addMana(t, e, 0, "R")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority after the raise floats: %+v", d)
	}
	opts = castOptions(t, e)
	kicked = -1
	for _, o := range opts {
		if o.Obj == spell && o.Mode == "kicked" {
			kicked = o.Index
		}
	}
	if kicked < 0 {
		t.Fatalf("the kicked cast withheld although {4} plus {R} pays its credited total: %+v", opts)
	}
	replayCheck(t, e, cfg)
}
