package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// exileCtrScen builds the counters-on-exiled-cards pool, with the card ids by
// name.
func exileCtrScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "exileonleavectr")
	deck := []cardset.Entry{
		{Name: "Dauthi Voidwalker", Count: 1},
		{Name: "Draugr Necromancer", Count: 1},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Doom Blade", Count: 1},
		{Name: "Swamp", Count: 1},
		{Name: "Forest", Count: 1},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Forest"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// TestExileOnLeaveCounterStore: a permanent dying under Dauthi Voidwalker is
// exiled with a VOID counter, the counter survives in exile (a version-matched
// cold record), and Dauthi's own ChooseCard filter reads it there. The compared
// values differ: the bear is on the battlefield with no counter before the
// move, and a counter-less exiled card does not match the filter.
func TestExileOnLeaveCounterStore(t *testing.T) {
	sc, id := exileCtrScen(t)
	s, k := sc.s, sc.k
	sc.perm(id["Dauthi Voidwalker"], 0)
	bear := sc.perm(id["Grizzly Bears"], 1)
	if z := s.objs[bear].zone; z != kop.ZoneBattlefield {
		t.Fatalf("precondition: bear is in zone %d, want the battlefield", z)
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 0 {
		t.Fatalf("precondition: bear has %d exile VOID counters", n)
	}

	s.leave(k, sc.slot(bear), kop.ZoneGraveyard, false)
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the dying bear is in zone %d, want exile", z)
	}
	if n := s.exileCounters(bear, kop.CtrVoid); n != 1 {
		t.Fatalf("the exiled bear has %d VOID counters, want 1", n)
	}
	if n := s.exileCounters(bear, kop.CtrIce); n != 0 {
		t.Fatalf("the exiled bear has %d ICE counters, want 0", n)
	}

	// Dauthi's activated ability is a filtered ChooseCard over Exile; its
	// built filter must select the exiled, VOID-countered opponent card.
	dc := id["Dauthi Voidwalker"]
	var f kop.FilterID
	for _, ab := range k.t.Cards[dc].Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpChooseCard {
				f = in.F
			}
		}
	}
	if f == 0 {
		t.Fatal("Dauthi Voidwalker has no built ChooseCard filter")
	}
	if !s.matchCardGen(k, f, s.objs[bear].card, 1, 0, bear) {
		t.Fatal("Dauthi's ChooseCard filter does not match the exiled bearer with a VOID counter")
	}
	// a second card in exile with no counter is not a candidate: the filter's
	// counter read is doing the work, not the zone/owner clauses alone
	plain := sc.obj(id["Grizzly Bears"], 1)
	s.objs[plain].zone = kop.ZoneExile
	if s.matchCardGen(k, f, s.objs[plain].card, 1, 0, plain) {
		t.Fatal("Dauthi's ChooseCard filter matches an exiled card with no VOID counter")
	}
}

// TestExileOnLeaveCounterStoreAnyZone: a card put into a graveyard from a
// non-battlefield zone under Dauthi (a resolved spell, a discard, a mill)
// takes the any-zone path, exileAnyDest, and is exiled with the counter too.
func TestExileOnLeaveCounterStoreAnyZone(t *testing.T) {
	sc, id := exileCtrScen(t)
	s, k := sc.s, sc.k
	sc.perm(id["Dauthi Voidwalker"], 0)
	o := sc.obj(id["Grizzly Bears"], 1)
	s.objs[o].zone = kop.ZoneLibrary
	if n := s.exileCounters(o, kop.CtrVoid); n != 0 {
		t.Fatalf("precondition: the library card has %d exile VOID counters", n)
	}

	s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false, false)
	if z := s.objs[o].zone; z != kop.ZoneExile {
		t.Fatalf("the card put into a graveyard from the library is in zone %d, want exile", z)
	}
	if n := s.exileCounters(o, kop.CtrVoid); n != 1 {
		t.Fatalf("the any-zone exiled card has %d VOID counters, want 1", n)
	}
}

// TestExileOnLeaveCounterStoreDraugr: the battlefield-origin ICE counter takes
// the dieZone move and stays on the exiled card.
func TestExileOnLeaveCounterStoreDraugr(t *testing.T) {
	sc, id := exileCtrScen(t)
	s, k := sc.s, sc.k
	sc.perm(id["Draugr Necromancer"], 0)
	bear := sc.perm(id["Grizzly Bears"], 1)
	if n := s.exileCounters(bear, kop.CtrIce); n != 0 {
		t.Fatalf("precondition: bear has %d exile ICE counters", n)
	}

	s.leave(k, sc.slot(bear), kop.ZoneGraveyard, false)
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("the dying bear is in zone %d, want exile", z)
	}
	if n := s.exileCounters(bear, kop.CtrIce); n != 1 {
		t.Fatalf("the exiled bear has %d ICE counters, want 1", n)
	}

	// Draugr's may-play static is Affected$ Card.OppOwn+counters_GE1_ICE+
	// nonLand over Exile: its built filter must match the exiled ICE card.
	dc := id["Draugr Necromancer"]
	var f kop.FilterID
	for _, ab := range k.t.Cards[dc].Abilities {
		if ab.Static.Kind == kop.StatMayPlayFrom {
			f = ab.Static.Filter
		}
	}
	if f == 0 {
		t.Fatal("Draugr Necromancer has no built MayPlayFrom static")
	}
	if !s.matchMayPlayFilter(k, f, s.objs[bear].card, 1, 0, bear, statSrc{}, nil) {
		t.Fatal("Draugr's may-play filter does not match the exiled bear with an ICE counter")
	}
	plain := sc.obj(id["Grizzly Bears"], 1)
	s.objs[plain].zone = kop.ZoneExile
	if s.matchMayPlayFilter(k, f, s.objs[plain].card, 1, 0, plain, statSrc{}, nil) {
		t.Fatal("Draugr's may-play filter matches an exiled card with no ICE counter")
	}
}
