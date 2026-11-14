package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// beholdScen is a Kinsbaile Aspirant ({W}, Behold<1/Kithkin> or {2}) in
// hand with plains untapped Plains and a Kithkin Billyrider (a behold
// candidate) on the battlefield. The behold lockstep never completes a
// {N}-part cast (gorge's pool-only castable leaves it only undone), so this
// is the only test of that payment.
func beholdScen(t *testing.T, cfg Config, plains int) (*scen, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Kinsbaile Aspirant", Count: 2},
		{Name: "Kithkin Billyrider", Count: 2},
		{Name: "Plains", Count: 4},
	}
	set := cardsettest.Set(t, "behold")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], cfg)
	sc.lands(0, id["Plains"], plains)
	ka := sc.hand(id["Kinsbaile Aspirant"], 0)
	bill := sc.perm(id["Kithkin Billyrider"], 0)
	return sc, ka, bill
}

// tappedAndFloating counts p's tapped permanents and the mana in p's pool.
func (sc *scen) tappedAndFloating(p uint8) (tapped, floating int) {
	for m := sc.s.ctrlM[p]; m.any(); m = m.rest() {
		if sc.s.perms[m.first()].flags&pfTapped != 0 {
			tapped++
		}
	}
	for _, n := range sc.s.pl[p].pool {
		floating += int(n)
	}
	return tapped, floating
}

// TestBeholdManaPartCompletes: choosing the {2} part of a Behold carrier's
// either-or additional cost pays it in mana, poses no behold ask and
// beholds nothing; with Config.GorgeAltAdd false an unpayable {2} part is
// not listed, with it both parts are.
func TestBeholdManaPartCompletes(t *testing.T) {
	sc, ka, bill := beholdScen(t, Config{NoElide: true, GorgeAltAdd: true}, 3)
	sc.do(t, AkCast, TCode(ka), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 2 || d.Options[0].Target() != 0 || d.Options[1].Target() != 1 {
		t.Fatalf("altadd decision %v, want the behold part then the {2} part", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[1])
	for i := 0; i < 8; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		if d.Kind == DkChoose && d.Sub == ChCostBehold {
			t.Fatalf("the {2} part posed a behold ask %v", d.Options[:d.N])
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[ka].zone != kop.ZoneStack {
		t.Fatalf("the {2}-part cast did not reach the stack (zone %v)", sc.s.objs[ka].zone)
	}
	if !sc.onBF(bill) {
		t.Fatalf("the behold candidate moved")
	}
	if tapped, _ := sc.tappedAndFloating(0); tapped != 3 {
		t.Fatalf("%d Plains tapped, want 3 ({W} + {2})", tapped)
	}

	// one Plains: only the behold part is payable
	for _, both := range []bool{false, true} {
		sc, ka, _ := beholdScen(t, Config{NoElide: true, GorgeAltAdd: both}, 1)
		sc.do(t, AkCast, TCode(ka), scAny)
		d := sc.s.Step()
		want := 1
		if both {
			want = 2
		}
		if d.Kind != DkChoose || d.Sub != ChAltAdd || int(d.N) != want || d.Options[0].Target() != 0 {
			t.Fatalf("GorgeAltAdd=%v: altadd decision %v, want %d option(s) led by the behold part", both, d.Options[:d.N], want)
		}
	}
}

// TestBeholdUnpayableManaPart: choosing the {2} part with too little mana
// undoes the cast (CR 733.1) after gorge's CR 601.2g window: an untargeted
// carrier reaches the window, which taps the sources and leaves their mana
// floating (the behold lockstep without its policy steering diverged here).
func TestBeholdUnpayableManaPart(t *testing.T) {
	sc, ka, bill := beholdScen(t, Config{NoElide: true, GorgeAltAdd: true}, 2)
	sc.do(t, AkCast, TCode(ka), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd || d.N != 2 || d.Options[0].Target() != 0 || d.Options[1].Target() != 1 {
		t.Fatalf("altadd decision %v, want the behold part then the unpayable {2} part", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[1])
	if d := sc.s.Step(); d.Kind != DkPriority || sc.s.objs[ka].zone != kop.ZoneHand || !sc.onBF(bill) {
		t.Fatalf("the unpayable {2} part did not undo the cast")
	}
	if tapped, floating := sc.tappedAndFloating(0); tapped != 2 || floating != 2 {
		t.Fatalf("%d Plains tapped and %d mana floating, want both Plains tapped and {W}{W} floating", tapped, floating)
	}
}
