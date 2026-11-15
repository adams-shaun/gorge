package dirac

import (
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kcounters (ctr_kinds.go): the unfeaturized counter kinds and
// Station, on the generated station pool.

// TestFeatCounterLayoutFrozen pins the kernel feature layout the trained
// models consume: the featurized counter range, the deathtouch mark's
// index and every counter array's width stay where they were before the
// unfeaturized kinds (CtrCharge..) were added above CtrLevel.
func TestFeatCounterLayoutFrozen(t *testing.T) {
	pins := []struct {
		name      string
		got, want int
	}{
		{"kop.NumCounterKinds", int(kop.NumCounterKinds), 7},
		{"kop.NumCounterKinds2", int(kop.NumCounterKinds2), 13},
		{"FeatCtrDeathtouched", FeatCtrDeathtouched, 13},
		{"FeatCtrKinds", FeatCtrKinds, 14},
		{"kop.CtrLevel", int(kop.CtrLevel), 14},
		{"kop.CtrCharge", int(kop.CtrCharge), 15},
		{"len(ProjObj.Counters)", len(ProjObj{}.Counters), 13},
		{"sizeof(ProjObj.Counters)", int(unsafe.Sizeof(ProjObj{}.Counters)), 26},
		{"kop.CtrStash", int(kop.CtrStash), 12},
		{"kop.CtrShield", int(kop.CtrShield), 4},
	}
	for _, p := range pins {
		if p.got != p.want {
			t.Errorf("%s = %d, pinned %d: the feature layout is frozen (trained models read it)", p.name, p.got, p.want)
		}
	}
	for c := 0; c < int(kop.NumCounterKinds3); c++ {
		want := c < int(kop.NumCounterKinds2) || c >= int(kop.CtrCharge)
		if kop.IsCounterKind(c) != want {
			t.Errorf("IsCounterKind(%d) = %v", c, !want)
		}
	}
	// an unfeaturized kind is never seen (perm.cseen) or listed
	s := &State{}
	var p perm
	if !p.setCtrN(s, int(kop.CtrCharge), 3) || !p.setCtrN(s, int(kop.CtrStun), 1) {
		t.Fatal("no slot")
	}
	p.seeCtr(s)
	if m := p.ctrSeenMask(s); m != 1<<kop.CtrStun {
		t.Fatalf("seen mask %#x, want only STUN", m)
	}
	p.subCtrN(s, int(kop.CtrCharge), 3)
	p.subCtrN(s, int(kop.CtrStun), 1)
	if m := p.ctrSeenMask(s); m != 1<<kop.CtrStun {
		t.Fatalf("seen mask %#x after removal, want only STUN (gorge lists it at 0)", m)
	}
}

func stationScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "station")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the station table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
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
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// stationAb is the Station ability index of card c.
func stationAb(t *testing.T, sc *scen, c CardID) uint8 {
	t.Helper()
	for i, a := range sc.k.t.Cards[c].Abilities {
		if a.AFlags&kop.AfStation != 0 {
			return uint8(i)
		}
	}
	t.Fatalf("%s has no Station ability", sc.k.t.Cards[c].Name)
	return 0
}

func TestStationPutsChargeEqualToPower(t *testing.T) {
	sc, id := stationScen(t, "Warmaker Gunship", "Hill Giant", "Grizzly Bears")
	if !sc.k.gen.anyStation {
		t.Fatal("prep anyStation false on a table with a Station card")
	}
	ship := sc.perm(id["Warmaker Gunship"], 0)
	ab := stationAb(t, sc, id["Warmaker Gunship"])
	// no other creature: not offered (gorge: a candidate must exist)
	for _, a := range sc.classActs(t, ship) {
		if a == ab {
			t.Fatal("Station offered with no creature to tap")
		}
	}
	giant := sc.perm(id["Hill Giant"], 0)
	sc.do(t, AkActivate, TCode(ship), scAny)
	// the pick comes before the push, always asked even with one candidate
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChStation || d.N != 1 || d.Options[0].Source() != TCode(uint8(giant)) {
		t.Fatalf("decision %v sub %v n %d: want the one-candidate ChStation pick", d.Kind, d.Sub, d.N)
	}
	if sc.s.sp != 0 {
		t.Fatal("the Station ability is on the stack before its pick (gorge mints it after the tap)")
	}
	sc.s.Apply(d.Options[0])
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision %v after the pick, want priority (the ability built and paid)", d.Kind)
	}
	if sc.s.sp != 1 || sc.s.perms[sc.slot(giant)].flags&pfTapped == 0 {
		t.Fatalf("stack %d, giant tapped %v after the pick", sc.s.sp, sc.s.perms[sc.slot(giant)].flags&pfTapped != 0)
	}
	if e := &sc.s.stack[0]; e.trigObj != giant || e.ab != ab {
		t.Fatalf("entry trigObj %d ab %d", e.trigObj, e.ab)
	}
	// not at instant speed: nothing offered with the ability on the stack
	for _, a := range sc.classActs(t, ship) {
		if a == ab {
			t.Fatal("Station offered with a non-empty stack")
		}
	}
	sc.resolveAll(t)
	if n := sc.s.Counters(ship, kop.CtrCharge); n != 3 {
		t.Fatalf("%d charge counters, want the Hill Giant's power 3", n)
	}
	var po ProjObj
	sc.s.projObj(sc.k, ship, &po)
	if po.CountersX[0] != 3 || po.Counters != [kop.NumCounterKinds2]uint16{} {
		t.Fatalf("projection counters %v x %v", po.Counters, po.CountersX)
	}
	if m := sc.s.FeatCountersListed(ship, &po.Counters); m != 0 {
		t.Fatalf("listed %#x: an unfeaturized kind reached the feature view", m)
	}
}

// TestNoncreatureDiesTrigger: Necrosquito's battlefield-to-graveyard
// trigger over "another creature or artifact you control" fires for an
// artifact sacrificed as a cost (Golden Urn), as gorge's ChangesZone does
// (trig_diesnoncreature.go; found by the station lockstep once OIL lifted it).
func TestNoncreatureDiesTrigger(t *testing.T) {
	sc, id := stationScen(t, "Necrosquito", "Golden Urn")
	if !sc.k.gen.anyDiesNC {
		t.Fatal("prep anyDiesNC false with Necrosquito in the table")
	}
	sq := sc.perm(id["Necrosquito"], 0)
	urn := sc.perm(id["Golden Urn"], 0)
	if n := sc.s.Counters(sq, kop.CtrOil); n != 2 {
		t.Fatalf("%d oil counters as it enters, want 2", n)
	}
	sc.do(t, AkActivate, TCode(urn), scAny)
	sc.resolveAll(t)
	if sc.onBF(urn) {
		t.Fatal("the Urn was not sacrificed")
	}
	if n := sc.s.Counters(sq, kop.CtrOil); n != 3 {
		t.Fatalf("%d oil counters after the artifact died, want 3", n)
	}
}
