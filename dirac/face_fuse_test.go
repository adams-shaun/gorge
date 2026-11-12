package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Fuse (face_fuse.go) and aftermath (face_aftermath.go) on their fixture
// pools: the offers, the face each cast turns up and the zone each rests in.

// poolScen is facesScen over another fixture pool: one of each named card
// in a Plains deck, both hands emptied, seat 0 at its first main phase.
func poolScen(t *testing.T, pool string, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, pool)
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the %s table", n, pool)
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

// fusedOffered reports whether d offers o's fused cast.
func fusedOffered(d *Decision, o Obj) bool {
	for _, f := range faceCasts(d, o) {
		if f == FeatFuseFace {
			return true
		}
	}
	return false
}

// TestFuseOfferNeedsThePool: Ready // Willing's fused cast costs both halves
// ({2}{G}{W}{W}{B}, CR 702.102c). Off the lockstep surfaces the census of six
// untapped lands offers it beside the two halves; on them gorge's planner
// prices no fused cast, so it waits for a floating pool that pays it, and a
// pool one white short still withholds it.
func TestFuseOfferNeedsThePool(t *testing.T) {
	sc, id := poolScen(t, "fuse", "Ready", "Forest", "Swamp")
	front := id["Ready"]
	fd, ok := fusedDef(sc.k, front)
	if !ok || !sc.k.t.Cards[fd].FusedSpell || sc.k.t.Cards[fd].Face != 0 || sc.k.cards[fd].total != 6 {
		t.Fatalf("Ready's fused def: ok %v %+v", ok, sc.k.t.Cards[fd])
	}
	for _, c := range []string{"Forest", "Forest", "Plains", "Plains", "Swamp", "Swamp"} {
		sc.perm(id[c], 0)
	}
	o := sc.hand(front, 0)
	d := sc.s.Step()
	if got := faceCasts(&d, o); len(got) != 2 || got[0] != 1 || got[1] != FeatFuseFace {
		t.Fatalf("census surface: face casts %v, want [1 %d] (Willing alone, then both halves)", got, FeatFuseFace)
	}
	sc.s.cfg |= cfgGorgeUnpriced
	d = sc.s.Step()
	if fusedOffered(&d, o) {
		t.Fatal("lockstep surface: the fused cast offered with an empty pool")
	}
	pl := &sc.s.pl[0]
	pl.pool[kop.G], pl.pool[kop.W], pl.pool[kop.B], pl.pool[kop.C] = 1, 1, 1, 2
	d = sc.s.Step()
	if fusedOffered(&d, o) {
		t.Fatal("lockstep surface: the fused cast offered one white short")
	}
	pl.pool[kop.W] = 2
	d = sc.s.Step()
	if !fusedOffered(&d, o) {
		t.Fatalf("lockstep surface: no fused cast with the pool paying it (%v)", faceCasts(&d, o))
	}
}

// TestFusedCastResolvesBothHalves: Wear // Tear cast fused shows its front
// on the stack as the fused def (both halves' targets, CR 702.102d), destroys
// the artifact and then the enchantment, and rests in the graveyard as its
// front (CR 709.4).
func TestFusedCastResolvesBothHalves(t *testing.T) {
	sc, id := poolScen(t, "fuse", "Wear", "Ornithopter", "Glorious Anthem", "Mountain")
	front := id["Wear"]
	fd, _ := fusedDef(sc.k, front)
	orn := sc.perm(id["Ornithopter"], 1)
	anthem := sc.perm(id["Glorious Anthem"], 1)
	pl := &sc.s.pl[0]
	pl.pool[kop.R], pl.pool[kop.W], pl.pool[kop.C] = 1, 1, 1
	o := sc.hand(front, 0)
	d := sc.s.Step()
	if !fusedOffered(&d, o) {
		t.Fatalf("no fused cast offered: %v", faceCasts(&d, o))
	}
	sc.s.Apply(MakeAction(AkCast, uint8(o), FeatFuseFace, castFace))
	for i, want := range []Obj{orn, anthem} {
		d = sc.s.Step() // pushed (CR 601.2a), then each half's target asked
		if i == 0 && (sc.s.objs[o].card != fd || sc.s.objs[o].zone != kop.ZoneStack) {
			t.Fatalf("on the stack: card %d (fused %d) zone %d", sc.s.objs[o].card, fd, sc.s.objs[o].zone)
		}
		if d.Kind != DkTarget {
			t.Fatalf("decision %d, want a target for %d", d.Kind, want)
		}
		picked := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkTarget && Obj(a.Target()) == want {
				sc.s.Apply(a)
				picked = true
				break
			}
		}
		if !picked {
			t.Fatalf("target %d not offered: %v", want, d.Options[:d.N])
		}
	}
	sc.buildToStack(t)
	if p := pl.pool; p[kop.R] != 0 || p[kop.W] != 0 || p[kop.C] != 0 {
		t.Fatalf("pool after paying the fused cost: %v", p)
	}
	sc.resolveAll(t)
	if sc.onBF(orn) || sc.onBF(anthem) {
		t.Fatalf("Ornithopter on the battlefield %v, Glorious Anthem %v: both halves should resolve", sc.onBF(orn), sc.onBF(anthem))
	}
	if ob := &sc.s.objs[o]; ob.zone != kop.ZoneGraveyard || ob.card != front {
		t.Fatalf("after resolving: zone %d card %d, want the graveyard as the front %d", ob.zone, ob.card, front)
	}
}

// TestAftermathOnlyFromTheGraveyard: Dusk // Dawn in the hand offers Dusk
// alone, never Dawn; in the graveyard it offers Dawn (face 1), which
// returns the small creature card and is exiled as its front (CR 702.127a).
func TestAftermathOnlyFromTheGraveyard(t *testing.T) {
	sc, id := poolScen(t, "aftermath", "Dusk", "Savannah Lions", "Plains")
	front := id["Dusk"]
	if !aftermathHalf(sc.k, front) {
		t.Fatal("Dusk's right half is not an aftermath half")
	}
	for i := 0; i < 5; i++ {
		sc.perm(id["Plains"], 0)
	}
	inHand := sc.hand(front, 0)
	d := sc.s.Step()
	if got := faceCasts(&d, inHand); len(got) != 0 {
		t.Fatalf("hand: face casts %v, want none (Dawn only from the graveyard)", got)
	}
	lions := sc.grave(id["Savannah Lions"], 0, 1)[0]
	gy := sc.grave(front, 0, 1)[0]
	d = sc.s.Step()
	if got := faceCasts(&d, gy); len(got) != 1 || got[0] != 1 {
		t.Fatalf("graveyard: face casts %v, want [1]", got)
	}
	sc.s.Apply(MakeAction(AkCast, uint8(gy), 1, castFace))
	sc.buildToStack(t)
	if sc.s.objs[gy].card != sc.k.t.Cards[front].Faces[1] || sc.s.objs[gy].zone != kop.ZoneStack {
		t.Fatal("Dawn is not on the stack")
	}
	sc.resolveAll(t)
	if sc.s.objs[lions].zone != kop.ZoneHand {
		t.Fatalf("Savannah Lions in zone %d, want the hand", sc.s.objs[lions].zone)
	}
	if ob := &sc.s.objs[gy]; ob.zone != kop.ZoneExile || ob.card != front {
		t.Fatalf("after resolving: zone %d card %d, want exile as the front %d", ob.zone, ob.card, front)
	}
}
