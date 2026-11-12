package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 601.2f: X may be any value whose total cost — the mana cost with X's
// value, plus increases, minus reductions — the player can pay. A generic
// cost reduction (Mocking Sprite's "{1} less for instants and sorceries")
// is mana the announced X can spend, so the X announcement's ceiling rises
// by the composed reduction: an {X}{U} spell under three {1}-less reducers
// with two mana in the pool is payable at X = 4 ({4}{U} less {3} = {1}{U}),
// and the ask must offer it. The loop bound without the reduction (pool +
// graveyard + convoke credit + window + 1 = 3) would stop the menu at
// X = 3 and withhold X = 4, although CR 601.2b's announcement may be any
// value the payment can settle.
func TestXCeilingCountsGenericReduction(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	spell := mustCorpusCard(t, reg, "Breakthrough") // {X}{U}
	reducer := func(src string) *cards.Card {
		return card(t, "Name:"+src+"\nManaCost:1 U\nTypes:Creature Human Wizard\nPT:1/1\n"+
			"S:Mode$ ReduceCost | ValidCard$ Instant,Sorcery | Type$ Spell | Activator$ You | Amount$ 1\nOracle:x\n")
	}
	board := []*cards.Card{reducer("Mocking Sprite"), reducer("Baral, Chief of Compliance"), reducer("Archmage of Runes")}
	e := counterHands(t, []*cards.Card{spell}, nil, board, nil)
	e.G.Players[0].Pool[state.MU] = 2
	e.askPriority(0)
	spellID := handObj(t, e, 0, "Breakthrough")
	// Precondition: the spell is the cast being asked about, not another play.
	submitChoices(t, e, passToCast(t, e, spellID))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("no X announcement ask pending for Breakthrough: %+v", d)
	}
	if m := maxXOf(d.Options); m != 4 {
		t.Fatalf("Breakthrough ({X}{U}) under three {1}-less reducers with {U}{U}: max X = %d, want 4", m)
	}
	// The whole payable run is offered, not just the new top.
	for _, want := range []int{3, 4} {
		if !xOptionHas(d.Options, want) {
			t.Fatalf("X = %d not offered: %+v", want, d.Options)
		}
	}
}

func xOptionHas(options []decision.Option, x int) bool {
	for _, o := range options {
		if o.Kind == "x" && o.Amount == x {
			return true
		}
	}
	return false
}
