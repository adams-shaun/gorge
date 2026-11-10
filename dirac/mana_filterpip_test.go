package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// evendoScen: Evendo, Waking Haven ("{T}: Add {G}"; at 12 charge counters
// "{G}, {T}: Add {G} for each creature you control") untapped with charge
// counters on it, four Grizzly Bears, the given untapped lands, one mana of
// colour float in p0's pool (none for -1), and Giant Spider ({3}{G}) in hand.
// The plain taps and the pool make at most three: the spider is payable only
// through the filter (CR 605.3a, 601.2g).
func evendoScen(t *testing.T, charge uint16, float int, lands ...string) (sc *scen, evendo, spider Obj, ls []Obj) {
	t.Helper()
	sc, id := stationScen(t, "Evendo, Waking Haven", "Grizzly Bears", "Giant Spider", "Forest", "Plains")
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
	if float >= 0 {
		sc.s.pl[0].pool[float]++
	}
	spider = sc.hand(id["Giant Spider"], 0)
	if !sc.k.gen.anyFiltPip || !sc.k.gen.has(sc.s.cardAt(sc.slot(evendo)), gfFilt) {
		t.Fatal("Evendo's filter is not a one-pip filter of the prepared table")
	}
	if n := sc.s.Counters(evendo, kop.CtrCharge); n != int(charge) || sc.tapped(evendo) {
		t.Fatalf("Evendo holds %d charge counters (tapped %v), want %d untapped", n, sc.tapped(evendo), charge)
	}
	return sc, evendo, spider, ls
}

// TestEvendoFilterNeedsGreenFee: gorge lists the Station-granted "{G}, {T}"
// filter only while the live pool pays its {G} (an AddAbility$ grant is
// priced against the real pool, manaAbilityPayable). With {G} floating and a
// Plains, the filter's four G pay {3}{G}: the play is offered and the cast
// spends the floating {G} on the fee, taps Evendo for four and leaves the
// Plains untapped. A floating {W} cannot pay the green pip, so beside a Forest
// the spider is not offered (the generic-fee model would: W for the fee, four
// G and the Forest's), and with nothing floating the filter is no mana ability
// of gorge's at all, so Forest and Plains do not reach it either.
// (Each verdict was read from gorge's PotentialPaymentPlans on the same
// board.)
func TestEvendoFilterNeedsGreenFee(t *testing.T) {
	sc, evendo, spider, ls := evendoScen(t, 12, kop.G, "Plains")
	acts := sc.castOptions(spider)
	if len(acts) != 1 {
		t.Fatalf("Giant Spider cast options %v with {G} floating beside Evendo's filter and a Plains, want one", acts)
	}
	sc.s.Apply(acts[0])
	sc.s.Step()
	if !sc.tapped(evendo) {
		t.Error("Evendo untapped after the cast: the filter made the spider's mana")
	}
	if sc.tapped(ls[0]) {
		t.Error("Plains tapped: the filter's four G pay {3}{G} alone")
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the cast, want empty (the floating {G} paid the fee)", pool)
	}
	if sc.s.sp != 1 {
		t.Errorf("stack %d after the cast, want the spider", sc.s.sp)
	}

	for _, c := range []struct {
		name  string
		float int
		lands []string
	}{
		{"{W} floating, Forest", kop.W, []string{"Forest"}},
		{"nothing floating, Forest and Plains", -1, []string{"Forest", "Plains"}},
	} {
		sc, _, spider, ls := evendoScen(t, 12, c.float, c.lands...)
		if len(ls) != len(c.lands) || sc.s.pl[0].pool[kop.G] != 0 {
			t.Fatalf("%s: board not built", c.name)
		}
		if acts := sc.castOptions(spider); len(acts) != 0 {
			t.Errorf("%s: Giant Spider offered %v; gorge lists no filter without {G} in the pool", c.name, acts)
		}
	}
}

// TestEvendoFilterGatedOnStation: the filter is Evendo's only while it holds
// 12 charge counters (CR 702.184, 613.1f). With {G} floating and a Plains the
// spider is offered at 12 and 13 counters, not at 11 or none.
func TestEvendoFilterGatedOnStation(t *testing.T) {
	for _, c := range []struct {
		charge uint16
		offer  bool
	}{{12, true}, {13, true}, {11, false}, {0, false}} {
		sc, _, spider, _ := evendoScen(t, c.charge, kop.G, "Plains")
		if got := len(sc.castOptions(spider)) != 0; got != c.offer {
			t.Errorf("%d charge counters: Giant Spider offered %v, want %v", c.charge, got, c.offer)
		}
	}
}
