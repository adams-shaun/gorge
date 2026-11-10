package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// evendoScen: Evendo, Waking Haven ("{T}: Add {G}"; at 12 charge counters
// "{G}, {T}: Add {G} for each creature you control") with charge counters on
// it, four Grizzly Bears, the given lands, and Giant Spider ({3}{G}) in hand.
// The lands alone make Evendo's {G} plus theirs, three mana: the spider is
// payable only through the filter (CR 605.3a, 601.2g).
func evendoScen(t *testing.T, charge uint16, lands ...string) (sc *scen, evendo, spider Obj, ls []Obj) {
	t.Helper()
	sc, id := stationScen(t, "Evendo, Waking Haven", "Grizzly Bears", "Giant Spider", "Forest")
	evendo = sc.perm(id["Evendo, Waking Haven"], 0)
	if !sc.s.perms[sc.slot(evendo)].setCtrN(sc.s, int(kop.CtrCharge), int(charge)) {
		t.Fatal("no counter slot")
	}
	sc.s.perms[sc.slot(evendo)].flags &^= pfTapped // (a Planet enters tapped)
	for i := 0; i < 4; i++ {
		sc.perm(id["Grizzly Bears"], 0)
	}
	for _, n := range lands {
		ls = append(ls, sc.perm(id[n], 0))
	}
	spider = sc.hand(id["Giant Spider"], 0)
	return sc, evendo, spider, ls
}

// TestEvendoFilterNeedsGreenFee: with 12 charge counters, Evendo's "{G}, {T}"
// ability makes a G per creature, four. Forest funds its {G} and the spider's
// {3}{G} is paid from that pool: the play is offered (gorge's prefix search,
// the filter a paid census gap, potentialPrefixScript), and Forest and Evendo
// are tapped for it. With Plains beside Plains the fee cannot be paid in
// green (manaAbilityPayablePool is a coloured check): the generic-fee model
// would offer the play, gorge does not.
func TestEvendoFilterNeedsGreenFee(t *testing.T) {
	sc, evendo, spider, ls := evendoScen(t, 12, "Forest", "Plains")
	if !sc.k.gen.anyFiltPip || !sc.k.gen.has(sc.s.cardAt(sc.slot(evendo)), gfFilt) {
		t.Fatal("Evendo's filter is not a one-pip filter of the prepared table")
	}
	if n := sc.s.Counters(evendo, kop.CtrCharge); n != 12 {
		t.Fatalf("%d charge counters, want 12", n)
	}
	acts := sc.castOptions(spider)
	if len(acts) != 1 {
		t.Fatalf("Giant Spider cast options %v over Forest, Plains and Evendo's filter, want one", acts)
	}
	sc.s.Apply(acts[0])
	sc.s.Step()
	if !sc.tapped(evendo) || !sc.tapped(ls[0]) {
		t.Errorf("Evendo tapped %v, Forest tapped %v after the cast, want both (the fee and the filter)", sc.tapped(evendo), sc.tapped(ls[0]))
	}
	if sc.tapped(ls[1]) {
		t.Error("Plains tapped: the filter's four G pay {3}{G} alone")
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the cast, want empty", pool)
	}
	if sc.s.sp != 1 {
		t.Errorf("stack %d after the cast, want the spider", sc.s.sp)
	}

	// the fee is a green pip: white mana does not pay it
	sc, _, spider, _ = evendoScen(t, 12, "Plains", "Plains")
	if acts := sc.castOptions(spider); len(acts) != 0 {
		t.Fatalf("Giant Spider offered with Evendo over two Plains: the {G} fee has no green mana to pay it (%v)", acts)
	}
}

// TestEvendoFilterGatedOnStation: the filter is Evendo's only while it holds
// 12 charge counters (CR 702.184, 613.1f; gorge's census lists the granted
// ability while the grant is live). With 11 it is no census gap: the same
// board cannot pay the spider, with 12 it can.
func TestEvendoFilterGatedOnStation(t *testing.T) {
	for _, c := range []struct {
		charge uint16
		offer  bool
	}{{12, true}, {13, true}, {11, false}, {0, false}} {
		sc, _, spider, _ := evendoScen(t, c.charge, "Forest", "Plains")
		if got := len(sc.castOptions(spider)) != 0; got != c.offer {
			t.Errorf("%d charge counters: Giant Spider offered %v, want %v", c.charge, got, c.offer)
		}
	}
}
