package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckEachColourFee is the kernel-side half of TestEachColourFeeGap
// (dirac_test). Tarnation Vista's {1}, {T} each-colour ability is a census gap
// gorge's prefix search reaches only with the fee already floating: it never
// taps a land for it (the fee is no funding the census counts), and the fee
// is paid from the pool in C W U B R G order. Watchwolf ({G}{W}) beside a
// Grizzly Bears and a Savannah Lions is paid by nothing else.
func CheckEachColourFee(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	// vistaIn puts an untapped Vista that chose red onto seat 0's battlefield
	vistaIn := func(sc *scen) Obj {
		vista := sc.perm(id["Tarnation Vista"], 0)
		vsl := sc.slot(vista)
		d := sc.s.Step() // its as-enters colour choice: red, no {G} or {W} of its own
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkChoose && a.Mode() == 3 {
				sc.s.Apply(a)
				break
			}
		}
		if sc.s.perms[vsl].chosen != 3 {
			t.Fatalf("the Vista chose colour %d, want red", sc.s.perms[vsl].chosen)
		}
		sc.s.perms[vsl].flags &^= pfTapped // it enters tapped
		return vista
	}
	board := func(cfg Config, tender bool) (*scen, Obj, Obj, Obj) {
		sc := newScenTableLib(t, tbl, id["Mountain"], cfg)
		vista := vistaIn(sc)
		sc.perm(id["Grizzly Bears"], 0)
		sc.perm(id["Savannah Lions"], 0)
		sc.perm(id["Mountain"], 0)
		if tender {
			sc.perm(id["Bloom Tender"], 0)
		}
		return sc, vista, sc.hand(id["Watchwolf"], 0), Obj(0)
	}
	gapsAvail := func(s *State) avail {
		var a avail
		s.g.availX = availGaps
		s.availFor(s.k(), 0, &a)
		s.g.availX = 0
		return a
	}

	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}} {
		// (a) the card is both a chosen-colour source and a fee-paid each-colour gap
		sc, vista, wolf, _ := board(cfg, false)
		g := sc.k.gen
		vc := sc.s.objs[vista].card
		if g.eachCol(vc) != 1 || !sc.s.manaPCGen(sc.k, sc.slot(vista)).manaChosen {
			t.Fatalf("Vista: each-colour ability %d, chosen %v, want ability 1 beside the chosen {T}", g.eachCol(vc), sc.s.manaPCGen(sc.k, sc.slot(vista)).manaChosen)
		}
		if fee := sc.s.eachFee(sc.k, sc.slot(vista)); fee != 1 {
			t.Fatalf("Vista's each-colour fee = %d, want 1", fee)
		}
		if !sc.s.scriptGaps(sc.k, 0).has(sc.slot(vista)) {
			t.Fatal("the untapped Vista is no each-colour gap")
		}

		// (b) no fee floating: a land does not pay it. The gap prices no colour
		// (priced units net of the fee), and Watchwolf is not offered.
		a := gapsAvail(sc.s)
		if a.total != 2 { // Vista's chosen {R} and the Mountain: nothing of the each-colour set
			t.Errorf("cfg %+v: no fee floating, the gaps walk prices %d mana %v, want 2 (no {G} or {W})", cfg, a.total, a.mask[:a.n])
		}
		if offersCast(sc.s, wolf) {
			t.Errorf("cfg %+v: Watchwolf offered with no mana floating for the Vista's fee", cfg)
		}

		// (c) one {C} floating pays the fee: the Vista's {G}{W} pay Watchwolf
		sc.s.pl[0].pool[kop.C] = 1
		a = gapsAvail(sc.s)
		if !availHasMask(a, 1<<kop.G) || !availHasMask(a, 1<<kop.W) {
			t.Errorf("cfg %+v: a fee floating, the gaps walk prices %v, want the {G} and {W} units", cfg, a.mask[:a.n])
		}
		if !offersCast(sc.s, wolf) {
			t.Fatalf("cfg %+v: Watchwolf not offered with {C} floating for the Vista's fee", cfg)
		}
		// its prefix taps the Vista for its each-colour ability, the fee paid from the pool
		var want [kop.NumManaKinds]uint8
		want[kop.G], want[kop.W] = 1, 1
		sc2 := *sc
		sc2.s = sc.s.fork()
		sc2.s.scriptPrefix(sc.k, MakeAction(AkCast, uint8(wolf), 0, 0), 0)
		if got := sc2.s.pl[0].pool; got != want {
			t.Errorf("cfg %+v: the prefix leaves the pool %v, want {G}{W} (fee {C} paid)", cfg, got)
		}
		if !sc2.tapped(vista) {
			t.Errorf("cfg %+v: the prefix leaves the Vista untapped", cfg)
		}
		sc.s.unfork()

		// (d) the fee is paid C, W, U, B, R, G: a floating {R} pays it before a {G}
		// (the fee never eats a colour the cost needs while a {C} or an earlier
		// colour is there to spend)
		sc3, _, wolf3, _ := board(cfg, false)
		sc3.s.pl[0].pool[kop.R], sc3.s.pl[0].pool[kop.G] = 1, 1
		sc3.s.scriptPrefix(sc3.k, MakeAction(AkCast, uint8(wolf3), 0, 0), 0)
		var want3 [kop.NumManaKinds]uint8
		want3[kop.G], want3[kop.W] = 2, 1 // {R} paid the fee; {G} floated beside the Vista's {G}{W}
		if got := sc3.s.pl[0].pool; got != want3 {
			t.Errorf("cfg %+v: fee over {R}{G}: pool %v, want %v ({R} spent first)", cfg, got, want3)
		}
	}

	// (e) a Bloom Tender's free gap beside the Vista: gorge's search taps the
	// gaps in battlefield order, so the Vista (first) taps for its chosen {R}
	// with no ask (nothing floats to pay a fee), and the Tender's {G}{W} follow
	// (measured: engine/rules PotentialPaymentPlans on this board activates the
	// Vista, then the Tender)
	sc, vista, wolf, _ := board(Config{NoElide: true}, true)
	if !offersCast(sc.s, wolf) {
		t.Fatal("Watchwolf not offered with a Bloom Tender on the board")
	}
	sc.s.scriptPrefix(sc.k, MakeAction(AkCast, uint8(wolf), 0, 0), 0)
	if !sc.tapped(vista) {
		t.Error("the prefix left the Vista untapped: gorge's search taps it first")
	}
	var wantE [kop.NumManaKinds]uint8
	wantE[kop.R], wantE[kop.G], wantE[kop.W] = 1, 1, 1
	if got := sc.s.pl[0].pool; got != wantE {
		t.Errorf("the Vista then Tender prefix leaves the pool %v, want {R}{G}{W}", got)
	}

	// (f) the Tender alone is unchanged: its free {G}{W} pay Watchwolf
	sc = newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Savannah Lions"], 0)
	tender := sc.perm(id["Bloom Tender"], 0)
	wolf = sc.hand(id["Watchwolf"], 0)
	if !offersCast(sc.s, wolf) {
		t.Fatal("Watchwolf not offered with only a Bloom Tender")
	}
	sc.s.scriptPrefix(sc.k, MakeAction(AkCast, uint8(wolf), 0, 0), 0)
	var wantF [kop.NumManaKinds]uint8
	wantF[kop.G], wantF[kop.W] = 1, 1
	if !sc.tapped(tender) || sc.s.pl[0].pool != wantF {
		t.Errorf("the Tender-only prefix: tapped %v pool %v, want it tapped for {G}{W}", sc.tapped(tender), sc.s.pl[0].pool)
	}

	// (g) three Vistas, nothing floating: gorge's search is depth first with the
	// chosen {T} listed first, so the first two tap for {R} (the second's ask
	// is posed, its chosen {T} answered first) and the third's each-colour
	// ability is the first path that pays: its fee is one of those {R}, the
	// other floats. A first-fit tap (the second Vista's each-colour ability,
	// funded by the first's {R}) would stop with the third untapped.
	sc = newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	var vistas [3]Obj
	for i := range vistas {
		vistas[i] = vistaIn(sc)
	}
	sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Savannah Lions"], 0)
	wolf = sc.hand(id["Watchwolf"], 0)
	if !offersCast(sc.s, wolf) {
		t.Fatal("Watchwolf not offered with three Vistas")
	}
	sc.s.scriptPrefix(sc.k, MakeAction(AkCast, uint8(wolf), 0, 0), 0)
	var wantG [kop.NumManaKinds]uint8
	wantG[kop.R], wantG[kop.G], wantG[kop.W] = 1, 1, 1
	for i, v := range vistas {
		if !sc.tapped(v) {
			t.Errorf("three Vistas: Vista %d untapped after the prefix", i)
		}
	}
	if sc.s.pl[0].pool != wantG {
		t.Errorf("three Vistas: the prefix leaves the pool %v, want {R}{G}{W}", sc.s.pl[0].pool)
	}
}
