package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// yieldsNothingBoard is a board like the empty_dynamic_mana_gap repro's: a
// gate land whose second mana ability adds X mana of any one colour, X the
// creatures you control (Baldur's Gate's shape, zero here), a pay-life W/B
// land (Silent Clearing's shape), an {X}{B} instant destroying a creature
// with power X or less (Killing Glare's shape) in hand, and a 3/3 on the
// other side (Rotting Giant's shape). The second ability's amount resolves
// to zero now, so gorge's planner window never taps it and no announcement
// pays X = 3; gorge offers no cast. The cards are appended to the vanilla
// table (its Plains id is vanilla's); an Expr forces prepareGen, whose
// general dynamic-ability handling the shape reaches.
func yieldsNothingBoard(t *testing.T) (*scen, CardID, Slot, Slot) {
	t.Helper()
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = []kop.Expr{{Kind: kop.ExYourTurns, N: 1}} // inert: forces prepareGen
	youCreatures := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(vanilla.Table.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	powerLEX := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{
		Types: kop.TCreature, Pred: kop.PredPowerGEV, PredN: 2, PredV: kop.ValueRef{K: kop.VX},
	})
	glare := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Glare",
		Types: kop.TInstant, Colors: kop.ColB, Timing: kop.TimingInstant,
		Cost: kop.ManaCost{X: 1, Pips: [kop.NumManaKinds]uint8{kop.B: 1}},
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Targets: [kop.MaxTargets]kop.TargetSpec{{
			Kind: kop.TgtObject, Filter: powerLEX,
		}}, NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}})
	gate := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Gate", Types: kop.TLand, Timing: kop.TimingLand,
		Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 2}},
				ManaColors: kop.ColAny, ManaN: 1, AFlags: kop.AfManaDynamic,
				ManaV: kop.ValueRef{K: kop.VCountFilter, N: int16(youCreatures)}},
		}})
	wb := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Clearing", Types: kop.TLand, Timing: kop.TimingLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap | kop.CostPayLife, N: 1},
			ManaColors: kop.ColW | kop.ColB, ManaN: 1}}})
	giant := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Giant",
		Types: kop.TCreature, Colors: kop.ColB, Power: 3, Toughness: 3,
		Cost: kop.ManaCost{Generic: 1, Pips: [kop.NumManaKinds]uint8{kop.B: 1}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.hand(glare, 0)
	bgl := sc.slot(sc.perm(gate, 0))
	sl := sc.slot(sc.perm(wb, 0))
	sc.perm(giant, 1)
	return sc, glare, bgl, sl
}

// TestYieldsNothingDynamicManaCensusGap: a further dynamic mana ability whose
// non-literal amount resolves to zero now is no unit of the offer walk's
// per-tap census and gaps it not (gorge manaAbilityYieldsNothing), so the
// X-bound offer sweep stays exact and refuses the {X}{B} cast at every X
// whose target exists. Its {2} fee gapping the census offered the cast off
// the over-bound open pool.
func TestYieldsNothingDynamicManaCensusGap(t *testing.T) {
	sc, glare, bgl, sl := yieldsNothingBoard(t)
	s := sc.s
	s.prio = 0
	for _, p := range []Slot{bgl, sl} {
		if s.perms[p].flags&pfTapped != 0 {
			t.Fatalf("the probe's land is tapped")
		}
	}
	var d Decision
	s.priorityOptions(s.k(), &d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(s.pl[0].hand[0])) {
			t.Fatalf("the cast is offered (the census gap reopens)")
		}
	}
	if d.N == 0 {
		t.Fatalf("no options at all: the probe board does not reach the offer walk")
	}

	// the sweep's own inputs, as xBoundPayable computes them: the census is
	// complete (its per-tap units recorded) and the walk is not open.
	var pot, ex avail
	var pu [maxPotUnits]potUnit
	dd := payDesc{class: pdSpell, card: glare, src: NoSlot, og: restrOrigin{from: kop.ZoneHand}}
	if open := s.availPotentialFor(s.k(), 0, &pot, &dd, true, &pu); open {
		t.Fatalf("the potential walk is open (an empty ability still opens it)")
	}
	if !potUnitsAvail(&ex, &pu, int(s.pl[0].life)) {
		t.Fatalf("the per-tap census is incomplete (the gap marker stands)")
	}
	if ex.total != 2 {
		for i := 0; i < maxPotUnits && pu[i].mask != 0; i++ {
			t.Logf("unit %d: mask=%02x n=%d pain=%d", i, pu[i].mask, pu[i].n, pu[i].pain)
		}
		t.Fatalf("the per-tap census holds %d units, want 2 ({C} beside the W/B source)", ex.total)
	}
	// X = 1 is the largest payable announcement ({C} and one W/B mana, one
	// per tap); the Giant's power 3 is above every X up to it, and X = 2
	// needs a third mana the empty second ability cannot make.
	cx := s.k().t.Cards[glare].Cost.AtX(1)
	if !ex.affords(&cx, pipMaskOf(&cx), 0) {
		t.Fatalf("the census refuses X = 1 ({C} and the W/B source pay it)")
	}
	cx = s.k().t.Cards[glare].Cost.AtX(2)
	if ex.affords(&cx, pipMaskOf(&cx), 0) {
		t.Fatalf("the census prices X = 2 payable (the empty ability counts)")
	}
}
