package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// noGrantScen is grantTapExtraScen's board with no grant in its table: a
// generated table whose planner-deferred Taps watcher puts the script walk on
// (scriptAny: anyTapsDefer), two
// plain {C} lands and Test Ember ({1}) in hand, and no rite, so grantPC is
// empty.
func noGrantScen(t *testing.T) (*scen, Obj) {
	t.Helper()
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	mine := kop.FilterID(len(tbl.Filters) - 1)
	land := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Land", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColC, ManaN: 1},
	}})
	watch := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Taps Watch", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
		{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigTaps, Filter: mine, TapPlan: kop.TapPlanDefer}, Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}},
	}})
	ember := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Ember", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 1}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(land, 0)
	sc.perm(land, 0)
	sc.perm(watch, 0)
	return sc, sc.hand(ember, 0)
}

// TestCapSpentScratchIsReset: priorityOptions builds grantCap's cap only on a
// table with a grant search, and every other priority walks with a nil cap
// (cast_grantcap.go). The cap must still bind where it is built, the nil cap
// must read as an empty one, and a priority that filled a cap must leave
// nothing behind for the next priority on a table with no grant: it offers
// exactly what a fresh copy of its state offers.
func TestCapSpentScratchIsReset(t *testing.T) {
	gsc, emberO := grantTapExtraScen(t, true, false)
	gs, gk := gsc.s, gsc.k
	a := MakeAction(AkCast, TCode(emberO), 0, 0)
	if len(gk.gen.grantPC) == 0 || gs.g.availX != 0 {
		t.Fatalf("precondition: the grant board takes no grant search (grantPC %d, availX %d)", len(gk.gen.grantPC), gs.g.availX)
	}
	// precondition: the grant board's priority fills the cap (the grant
	// search refuses Test Ember)
	var probe State
	probe.CopyFrom(gs)
	var pd, pcap Decision
	pd.add(a)
	probe.grantCap(gk, &pd, 0, &pcap)
	if pcap.N == 0 || !hasOption(&pcap, a) {
		t.Fatalf("precondition: grantCap recorded %v, want the refused Test Ember", pcap.Options[:pcap.N])
	}

	nsc, _ := noGrantScen(t)
	ns, nk := nsc.s, nsc.k
	if len(nk.gen.grantPC) != 0 || !ns.scriptAny(nk) {
		t.Fatalf("precondition: the no-grant table has %d grant searches and script walk %v, want none and on", len(nk.gen.grantPC), ns.scriptAny(nk))
	}
	var fresh State
	fresh.CopyFrom(ns)

	// the priority that fills the cap: the refused play stays unoffered
	var gd Decision
	gs.priorityOptions(gk, &gd)
	if hasOption(&gd, a) {
		t.Fatal("the grant board's priority offers the Test Ember its grant search refused")
	}
	// the next priority, on a table with no grant, sees no cap: it offers
	// exactly what a fresh copy offers
	var nd, fd Decision
	ns.priorityOptions(nk, &nd)
	fresh.priorityOptions(nk, &fd)
	if nd.N == 0 || nd.N != fd.N || !sameOptions(&nd, &fd) {
		t.Fatalf("the no-grant priority offers %v, a fresh copy %v", nd.Options[:nd.N], fd.Options[:fd.N])
	}

	// the nil cap is the empty one, and a filled cap drops exactly its plays
	var wNil, wEmpty, wCap, empty, capOne Decision
	ns.scriptWalk(nk, &wNil, nil, 0, true)
	ns.scriptWalk(nk, &wEmpty, &empty, 0, true)
	if wNil.N != wEmpty.N || !sameOptions(&wNil, &wEmpty) {
		t.Fatalf("the nil-cap walk offers %v, the empty-cap walk %v", wNil.Options[:wNil.N], wEmpty.Options[:wEmpty.N])
	}
	if wEmpty.N == 0 {
		t.Fatal("precondition: the script walk offers nothing on the no-grant board, so the cap is not exercised")
	}
	capped := wEmpty.Options[0]
	capOne.add(capped)
	ns.scriptWalk(nk, &wCap, &capOne, 0, true)
	if hasOption(&wCap, capped) || int(wCap.N) != int(wEmpty.N)-1 {
		t.Fatalf("a cap holding %v leaves %v from %v, want exactly that play dropped", capped, wCap.Options[:wCap.N], wEmpty.Options[:wEmpty.N])
	}
}
