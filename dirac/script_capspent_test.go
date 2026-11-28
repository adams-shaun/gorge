package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// capSpentScen builds the TestFilterSearchDoesNotRepriceAGrantRefusedActivation
// board (Great Divide Guide's grant beside Rumble Arena's filter and Earthen
// Ally's Earthbend) with or without the grant's source in the deck: without
// it the table holds no grant search (grantPC is empty).
func capSpentScen(t *testing.T, grant bool) (*scen, Action) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Rumble Arena", Count: 1}, {Name: "Earthen Ally", Count: 1},
		{Name: "Bender's Waterskin", Count: 1}, {Name: "Meditation Pools", Count: 1}, {Name: "Jasmine Dragon Tea Shop", Count: 1},
		{Name: "White Lotus Hideout", Count: 1}, {Name: "Badgermole Cub", Count: 4},
		{Name: "Plains", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Island", Count: 20},
	}
	names := []string{"Rumble Arena", "Earthen Ally", "Jasmine Dragon Tea Shop",
		"Meditation Pools", "Plains", "Mountain", "White Lotus Hideout"}
	if grant {
		deck = append(deck, cardset.Entry{Name: "Great Divide Guide", Count: 1})
		names = append(names, "Great Divide Guide")
	}
	tbl, _, _, err := cardsettest.Synth(t, "std-tla").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	var ally Obj
	for _, n := range names {
		o := sc.perm(id[n], 0)
		if n == "Earthen Ally" {
			ally = o
		}
	}
	sc.s.pl[0].pool[kop.C] = 1
	ai := -1
	for i := range sc.k.t.Cards[id["Earthen Ally"]].Abilities {
		if sc.k.t.Cards[id["Earthen Ally"]].Abilities[i].Cost.Mana.Generic > 0 {
			ai = i
			break
		}
	}
	if ai < 0 {
		t.Fatal("Earthen Ally's Earthbend ability not found in the table")
	}
	return sc, MakeAction(AkActivate, TCode(ally), 0, uint8(ai))
}

// TestCapSpentScratchIsReset: priorityOptions builds grantCap's cap only on a
// table with a grant search, and every other priority walks with a nil cap
// (script_capspent.go). The cap must still bind where it is built, the nil cap
// must read as an empty one, and a priority that filled a cap must leave
// nothing behind for the next priority on a table with no grant: it offers
// exactly what a fresh copy of its state offers.
func TestCapSpentScratchIsReset(t *testing.T) {
	gsc, a := capSpentScen(t, true)
	gs, gk := gsc.s, gsc.k
	if len(gk.gen.grantPC) == 0 || gs.g.availX != 0 {
		t.Fatalf("precondition: the grant board takes no grant search (grantPC %d, availX %d)", len(gk.gen.grantPC), gs.g.availX)
	}
	// precondition: the grant board's priority fills the cap (the grant search
	// refuses the Earthbend within gorge's budget)
	var probe State
	probe.CopyFrom(gs)
	var pd, pcap Decision
	pd.add(a)
	probe.grantCap(gk, &pd, 0, &pcap)
	if pcap.N == 0 || !hasOption(&pcap, a) {
		t.Fatalf("precondition: grantCap recorded %d plays, want the refused Earthbend", pcap.N)
	}

	nsc, _ := capSpentScen(t, false)
	ns, nk := nsc.s, nsc.k
	if len(nk.gen.grantPC) != 0 {
		t.Fatalf("precondition: the no-grant board's table has %d grant searches, want none", len(nk.gen.grantPC))
	}
	var fresh State
	fresh.CopyFrom(ns)

	// the priority that fills the cap: the refused play stays unoffered
	var gd Decision
	gs.priorityOptions(gk, &gd)
	if hasOption(&gd, a) {
		t.Fatal("the grant board's priority offers the Earthbend its grant search refused")
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
