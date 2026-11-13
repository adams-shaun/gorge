package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kpaylifeScen: p0 holds Fumarole ({3}{B}{R}, PayLife<3>) with five
// untapped lands; p1 has a Hill Giant and a Swamp to target.
func kpaylifeScen(t *testing.T, cfg Config) (*scen, Obj, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Fumarole", Count: 1}, {Name: "Hill Giant", Count: 1},
		{Name: "Swamp", Count: 20}, {Name: "Mountain", Count: 20}}
	set := cardsettest.Set(t, "paylife")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, cfg)
	for _, n := range []string{"Swamp", "Swamp", "Mountain", "Mountain", "Mountain"} {
		sc.perm(id[n], 0)
	}
	giant := sc.perm(id["Hill Giant"], 1)
	swamp := sc.perm(id["Swamp"], 1)
	return sc, sc.hand(id["Fumarole"], 0), giant, swamp
}

func offersCast(s *State, o Obj) bool {
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == o {
			return true
		}
	}
	return false
}

// TestPayLifeSpellUnplanned: gorge's cast planner declines a spell whose
// own Cost$ has a life part (payment_plan.go: pay.PlanNonManaAdmissible
// "cost:life" -> "shape:additional_cost"), so on the autopay surface
// Fumarole is offered only when the floating pool pays it; the kernel
// surface (no gorge config) offers it from the untapped lands.
func TestPayLifeSpellUnplanned(t *testing.T) {
	sc, fum, _, _ := kpaylifeScen(t, Config{GorgeAutopaySurface: true, NoElide: true})
	if sc.s.FeatPlannedCast(0, fum, 0) {
		t.Fatalf("FeatPlannedCast(Fumarole) = true; gorge's planner declines a life cost")
	}
	if offersCast(sc.s, fum) {
		t.Fatalf("autopay surface offers Fumarole with an empty pool")
	}
	sc, fum, _, _ = kpaylifeScen(t, Config{NoElide: true})
	if !offersCast(sc.s, fum) {
		t.Fatalf("kernel surface does not offer Fumarole")
	}
	sc, fum, _, _ = kpaylifeScen(t, Config{GorgeAutopaySurface: true, NoElide: true})
	sc.s.pl[0].life = 2
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
	if offersCast(sc.s, fum) {
		t.Fatalf("Fumarole offered with 2 life")
	}
}

// TestPayLifeSpellLowered: under Config.GorgeLowering the oracle lowers
// the cast (gorge's potential planner prices the mana part): the witness
// taps the five lands into the pool before the targets are chosen (CR
// 601.2c after the lowering's mana activations), and the payment (CR
// 601.2h) spends that pool and pays the 3 life.
func TestPayLifeSpellLowered(t *testing.T) {
	sc, fum, giant, swamp := kpaylifeScen(t, Config{GorgeLowering: true, NoElide: true})
	sc.do(t, AkCast, TCode(fum), scAny)
	pool, _ := sc.s.PoolOf(0)
	n := 0
	for _, u := range pool {
		n += int(u)
	}
	if n != 5 {
		t.Fatalf("pool before the targets holds %d mana (%v), want the 5 lowered", n, pool)
	}
	if sc.s.LifeOf(0) != 20 {
		t.Fatalf("life %d before payment, want 20", sc.s.LifeOf(0))
	}
	for i := 0; i < 2; i++ {
		d := sc.s.Step()
		if d.Kind != DkTarget {
			t.Fatalf("decision %d after the cast, want a target", d.Kind)
		}
		want := swamp
		for j := 0; j < int(d.N); j++ {
			if Obj(d.Options[j].Target()) == giant {
				want = giant
			}
		}
		sc.do(t, AkTarget, scAny, TCode(want))
	}
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision %d after the targets, want priority", d.Kind)
	}
	if sc.s.LifeOf(0) != 17 {
		t.Fatalf("life %d after payment, want 17", sc.s.LifeOf(0))
	}
	pool, _ = sc.s.PoolOf(0)
	if pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("pool %v after payment, want empty", pool)
	}
	if sc.s.sp != 1 {
		t.Fatalf("stack %d, want Fumarole", sc.s.sp)
	}
}

// TestAbortCountPerCard: gorge counts no-progress aborts per card
// (abortCast's castAborts), so a card reversed twice stays held out after
// another card's reversal, and a resolution ends the window.
func TestAbortCountPerCard(t *testing.T) {
	sc, fum, _, _ := kpaylifeScen(t, Config{NoElide: true})
	s := sc.s
	other := sc.obj(s.objs[fum].card, 0)
	s.heldTurn, s.heldStep, s.heldTouch = uint16(s.turn), s.step, s.touchN
	rev := func(o Obj) {
		noProg := s.noProgress()
		s.abortCount(o, noProg)
		if s.heldObj == o && noProg {
			s.heldN++
		} else {
			s.heldObj, s.heldN = o, 1
		}
		s.heldTurn, s.heldStep, s.heldTouch = uint16(s.turn), s.step, s.touchN // reverseCast's snapshot, after abortCount's marks
	}
	rev(fum)
	rev(fum)
	rev(other)
	if !s.castHeldOut(fum) {
		t.Fatalf("Fumarole not held out after its second abort and another card's first")
	}
	if s.castHeldOut(other) {
		t.Fatalf("a card aborted once is held out")
	}
	rev(other)
	if !s.castHeldOut(other) || !s.castHeldOut(fum) {
		t.Fatalf("both cards aborted twice must be held out")
	}
	s.heldTurn = ^uint16(0) // a resolution (pass)
	if s.castHeldOut(fum) || s.castHeldOut(other) {
		t.Fatalf("held out after progress")
	}
	rev(fum)                                                               // the first abort after progress opens a new window: the old counts are stale
	s.heldTurn, s.heldStep, s.heldTouch = uint16(s.turn), s.step, s.touchN // (reverseCast's snapshot)
	if s.castHeldOut(other) || s.castHeldOut(fum) {
		t.Fatalf("stale counts survived a new window")
	}
}
