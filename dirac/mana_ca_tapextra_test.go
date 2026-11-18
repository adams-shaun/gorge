package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// caStallBoard builds the tapsdefer board whose Combo Any arming card (the
// Birds of Paradise rewritten to "{T}: Add two mana in any combination of
// colours", kop.AfManaChoice) stays in hand — a sick creature is never a gap —
// and whose only Combo Any gap is a land with that ability as its second mana
// ability beside a plain "{T}: Add {C}" (its census gap, gfGapOther, is the
// Combo Any one, so caGaps lists it and caActivate taps it). Judge of Currents carries a fixed StatTapExtra (+1
// {R} on every tap of its controller's permanents), whose vector the Combo Any
// activation's model does not add (caActivate ends in tapExtraStall: the tap's
// srcExtra is the +1 {R}, the modelled production is the answered colours
// alone). It returns the scenario, the cast card in hand, and the second land's
// slot.
func caStallBoard(t *testing.T) (*scen, Obj, Slot) {
	t.Helper()
	var ember, dual CardID
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		ab := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, A: int16(kop.R), B: 1}}
		ab.Static.Filter = kop.FilterID(len(tbl.Filters))
		tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Ctrl: kop.CtrlYou})
		j := &tbl.Cards[id["Judge of Currents"]]
		j.Abilities = append(append([]kop.Ability(nil), j.Abilities...), ab)
		// the Birds arm gen.anyComboAny from hand (a sick creature is never a
		// gap, so the Dual Core below is the search's only Combo Any gap)
		rewriteMana(tbl, id["Birds of Paradise"], func(a *kop.Ability) {
			a.AFlags = kop.AfManaChoice
			a.ManaColors, a.ManaN = kop.ColAny, 2
		})
		caCost := kop.Cost{Flags: kop.CostTap}
		dual = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Dual Core", Types: kop.TLand, Abilities: []kop.Ability{
			{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
			{Kind: kop.AbMana, AFlags: kop.AfManaChoice, Cost: caCost, ManaColors: kop.ColAny, ManaN: 2},
		}})
		ember = CardID(len(tbl.Cards))
		emberCost := kop.ManaCost{}
		emberCost.Pips[kop.W] = 2
		emberCost.Pips[kop.U] = 2
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ember", Types: kop.TInstant, Cost: emberCost})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Judge of Currents"], 0)
	d := sc.perm(dual, 0)
	emberO := sc.hand(ember, 0)
	return sc, emberO, sc.slot(d)
}

// TestCaScriptStallPropagates: the Combo Any search's only payment line is the
// Dual Core's tap (its plain "{T}: Add {C}" is manaPCAt's view, manaAb >= 0),
// which brings the carrier's +1 {R} that the activation's model does not add —
// caActivate ends in tapExtraStall (StallTapExtra). The cast ({W}{W}{U}{U}) is past every answer's two mana, so
// every line of the search fails and caScript declares the latched stall on
// the real state instead of dropping the play as "no payment": the game draws
// as the unmodelled stall instead of silently losing the cast gorge offers
// (TestFiltScriptStallPropagates's shape).
func TestCaScriptStallPropagates(t *testing.T) {
	sc, emberO, dual := caStallBoard(t)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(emberO), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	if !s.caGaps(k, 0).has(dual) {
		t.Fatal("precondition: the second land is no Combo Any gap")
	}
	if s.caScript(k, a, 0, false) {
		t.Fatal("the search paid the cast, want its only line to stall")
	}
	if !s.Stalled() || s.StallReason() != StallTapExtra {
		t.Fatalf("the search dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallTapExtra))
	}
}
