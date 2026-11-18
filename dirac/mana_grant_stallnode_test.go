package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// grantTapExtraScen: seat 0 holds a pain land (a plain "{T}: Add {C}" beside a
// painful "{T}, Pay 1 life: Add {W}", pc.painCol) that a rite also grants
// "{T}: Add {U}" (kop.AfLatent granted by kop.StatContinuous), and a carrier
// whose reflected StatTapExtra (Judge of Currents' shape, kop.StTapReflect)
// reaches every tap of its controller's permanents. A source with a painful
// alternative is not one plain tap (tapExtraPlain: pc.painCol), so the
// reflected extra prices nothing on it and every tap of the land stalls by
// name (StallTapExtra, srcExtraPick) — the grant search's pain branch taps it
// (mana_grantpain.go painBranch).
func grantTapExtraScen(t *testing.T, grant, pain bool) (*scen, Obj) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	lands := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	plain := CardID(len(tbl.Cards))
	landAb := []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1}}
	if pain {
		landAb = append(landAb, kop.Ability{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostPayLife, N: 1}, ManaColors: kop.ColW, ManaN: 1})
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Pain Land", Types: kop.TLand, Abilities: landAb})
	rite := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Split Rite", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: lands, Grant: 2}},
		{Kind: kop.AbMana, AFlags: kop.AfLatent, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColU, ManaN: 1},
	}})
	judge := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Judge", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatTapExtra, B: 1, StFlags: kop.StTapReflect, Filter: mine}},
	}})
	watch := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Taps Watch", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigTaps, Filter: mine}, Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}},
	}})
	ember := CardID(len(tbl.Cards))
	emberCost := kop.ManaCost{}
	emberCost.Pips[kop.C] = 1
	emberCost.Pips[kop.U] = 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ember", Types: kop.TInstant, Cost: emberCost})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if grant {
		sc.perm(rite, 0)
	}
	land := sc.perm(plain, 0)
	if !sc.k.gen.anyTapExtra {
		t.Fatal("precondition: the table has no tap-extra carrier")
	}
	if grant && sc.s.grantIdx(sc.k, sc.slot(land)) < 0 {
		t.Fatal("precondition: the split grant does not reach the land")
	}

	sc.perm(watch, 0)
	sc.perm(judge, 0)
	emberO := sc.hand(ember, 0)
	return sc, emberO
}

// TestGrantSrchStallPropagates: on the grantTapExtraScen board the cast of
// Test Ember ({C}{U}) is past the printed census (the land's plain {C} and
// painful {W} pay neither the {U} of the cast nor its second unit beside the
// life; the potential walk's bound counts the granted {U} beside the plain
// {C}, so grantCap's walk loop searches it), and every line of the grant
// search's branches into the land latches StallTapExtra in the fork
// (painBranch's srcExtraPick: a painful source's reflected extra prices
// nothing; the tap's Taps watcher's pending ask settles through tapXNext,
// which a latched fork's stepGen refuses). The search reports the stall and
// declares nothing on the real state (grantSearch; a caller that keeps
// searching may still price the play), and the drop site declares it where it
// finally drops the play (grantCap's walk loop, the cap that keeps
// scriptOffers from repricing it): the game draws as the unmodelled stall
// instead of silently losing the cast gorge offers (TestTapXScriptStallPropagates's
// mechanism, TestFiltScriptStallPropagates's).
func TestGrantSrchStallPropagates(t *testing.T) {
	sc, emberO := grantTapExtraScen(t, true, true)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(emberO), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	cost, ex, need, ok := s.grantPlayCost(k, 0, a, true)
	if !ok {
		t.Fatal("precondition: grantPlayCost refused the cast")
	}
	// the search: its only route stalls, the search reports the stall and
	// declares nothing on the real state (the declaration is the drop site's)
	var g grantSrch
	if s.grantSearch(k, 0, &cost, ex, s.grantPayDesc(k, a), need, &g) {
		t.Fatal("the search found a payment path, want its only line to stall")
	}
	if g.stall != StallTapExtra {
		t.Fatalf("the search reported the failed line as plain no-payment: stall %s, want %s",
			StallName(g.stall), StallName(StallTapExtra))
	}
	if s.Stalled() {
		t.Fatalf("the search declared the stall itself (want the drop site's): %s", StallName(s.StallReason()))
	}
	// the cap: the play is dropped and the latched stall declared on the real
	// state, not silently swallowed
	var d, capSpent Decision
	d.add(a)
	s.grantCap(k, &d, 0, &capSpent)
	if !hasOption(&capSpent, a) {
		t.Fatal("the cap did not record the play its grant search refused")
	}
	if hasOption(&d, a) {
		t.Fatal("the cap kept the play whose only payment path stalls")
	}
	if !s.Stalled() || s.StallReason() != StallTapExtra {
		t.Fatalf("the cap dropped the play silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallTapExtra))
	}
}

// TestGrantSearchPricedLinesStayQuiet: a search whose every line prices its
// tap (the land's plain {C} and the granted {U} under the reflected carrier
// are one plain tap, the extra one more mana of the pick) fails the nine-pip
// cost without a stall: g.stall stays StallNone and nothing is declared on the
// real state, so the drop sites' g.stall arm does not fire on a plain refusal
// (a board with no candidate at all never forks either, grantSearch's
// g.n == 0 arm).
func TestGrantSearchPricedLinesStayQuiet(t *testing.T) {
	sc, emberO := grantTapExtraScen(t, true, false)
	s, k := sc.s, sc.k
	cost := kop.ManaCost{}
	cost.Pips[kop.G] = 9
	var g grantSrch
	if s.grantSearch(k, 0, &cost, NoSlot, s.grantPayDesc(k, MakeAction(AkCast, TCode(emberO), 0, 0)), [kop.NumManaKinds]bool{}, &g) {
		t.Fatal("the search paid a nine-pip cost")
	}
	if g.stall != 0 {
		t.Fatalf("a search whose lines price every tap reported %s", StallName(g.stall))
	}
	if s.Stalled() {
		t.Fatalf("the search latched %s on the real state", StallName(s.StallReason()))
	}
}
