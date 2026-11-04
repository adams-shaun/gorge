package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestP1P1Past255: a +1/+1 count is 16-bit, so a Mossborn Hydra-style
// doubling past 255 keeps growing the creature's P/T (gorge's counters are
// int32) and the last known information keeps the full count.
func TestP1P1Past255(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Healer's Hawk", Count: 1}})
	o := sc.perm(id["Healer's Hawk"], 0)
	sl := sc.slot(o)
	p := &sc.s.perms[sl]
	p.addCtrN(sc.s, int(kop.CtrP1P1), 300)
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 300 {
		t.Fatalf("+1/+1 count %d, want 300", n)
	}
	pw, tg := sc.s.pt(sc.k, sl)
	if pw != 301 || tg != 301 {
		t.Fatalf("P/T %d/%d, want 301/301 (Healer's Hawk is 1/1)", pw, tg)
	}
	if n := sc.s.lkiCounter(sc.s.snapLKI(o), kop.CtrP1P1); n != 300 {
		t.Fatalf("last known +1/+1 count %d, want 300", n)
	}
	if n := sc.s.creatureCounters(sc.k, 0); n != 300 {
		t.Fatalf("creature counters %d, want 300 (Quilled Greatwurm's six-counter cost)", n)
	}
}

// TestDoubleCountersPast255: a Mossborn Hydra-style doubling under Doubling
// Season adds twice its (already 16-bit) +1/+1 count. replCounters must keep
// doubling past 255 rather than stop at the old 8-bit bound: 300 becomes
// 600 added, for 900 total.
func TestDoubleCountersPast255(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Healer's Hawk", Count: 1}, {Name: "Doubling Season", Count: 1}})
	o := sc.perm(id["Healer's Hawk"], 0)
	sc.perm(id["Doubling Season"], 0)
	sl := sc.slot(o)
	p := &sc.s.perms[sl]
	p.addCtrN(sc.s, int(kop.CtrP1P1), 300)
	if got := sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 300); got != 600 {
		t.Fatalf("replCounters(300) = %d, want 600 (Doubling Season)", got)
	}
	p.addCtrN(sc.s, int(kop.CtrP1P1), sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 300))
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 900 {
		t.Fatalf("doubled +1/+1 count %d, want 900", n)
	}
}

// TestDoubleCountersTwoSeasons: every Doubling Season doubles, even when
// the current count is already large. With two seasons a count of 8000
// adds 32000 (8000 -> 16000 -> 32000), for 40000 total; stopping after the
// first double would add only 16000. A placement whose result passes the
// 16-bit storage bound is pair 163's stall (TestCounterOverflowStalls).
func TestDoubleCountersTwoSeasons(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Healer's Hawk", Count: 1}, {Name: "Doubling Season", Count: 2}})
	o := sc.perm(id["Healer's Hawk"], 0)
	sc.perm(id["Doubling Season"], 0)
	sc.perm(id["Doubling Season"], 0)
	sl := sc.slot(o)
	p := &sc.s.perms[sl]
	p.addCtrN(sc.s, int(kop.CtrP1P1), 8000)
	if got := sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 8000); got != 32000 {
		t.Fatalf("replCounters(8000) = %d, want 32000 (two Doubling Seasons)", got)
	}
	p.addCtrN(sc.s, int(kop.CtrP1P1), sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 8000))
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 40000 {
		t.Fatalf("doubled +1/+1 count %d, want 40000", n)
	}
}

// TestCounterOverflowStalls: a placement past the 16-bit storage bound is a
// declared capacity stall (StallCounters), not a silent clamp: gorge's count
// is unbounded (Bristly Bill, Spine Sower's doubling reached 81920, pair
// 163), so past 0xffff no stored value can agree with gorge and the game
// stops at the next step boundary. The stored count still saturates, so the
// state stays self-consistent until the stall surfaces.
func TestCounterOverflowStalls(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Healer's Hawk", Count: 1}, {Name: "Doubling Season", Count: 2}})
	o := sc.perm(id["Healer's Hawk"], 0)
	sc.perm(id["Doubling Season"], 0)
	sc.perm(id["Doubling Season"], 0)
	sl := sc.slot(o)
	p := &sc.s.perms[sl]
	p.addCtrN(sc.s, int(kop.CtrP1P1), 20000)
	if got := sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 20000); got != 80000 {
		t.Fatalf("replCounters(20000) = %d, want 80000 (two Doubling Seasons)", got)
	}
	p.addCtrN(sc.s, int(kop.CtrP1P1), sc.s.replCounters(sc.k, sl, int(kop.CtrP1P1), 20000))
	if n := p.ctrLow(int(kop.CtrP1P1)); n != 0xffff {
		t.Fatalf("stored +1/+1 count %d, want 65535 (saturating store)", n)
	}
	if r := sc.s.StallReason(); r != StallCounters {
		t.Fatalf("stall reason %d, want StallCounters (%d)", r, StallCounters)
	}
}

// TestGrantedFlashbackThrice: a graveyard card granted flashback by three
// Sphinx of Forgotten Lore (one a Self-Reflection token copy) reports the
// keyword three times, as gorge lists it once per grant; a fourth grant
// saturates.
func TestGrantedFlashbackThrice(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Inspiration from Beyond", Count: 1}})
	o := sc.obj(id["Inspiration from Beyond"], 0)
	sc.s.place(o, kop.ZoneGraveyard, false)
	bit := bits.TrailingZeros32(uint32(kop.KwFlashback))

	sc.s.grantFB(sc.k, o)
	if g, rep := sc.s.FeatGrantedOffRep(o); g != kop.KwFlashback || rep[bit] != 0 {
		t.Fatalf("one grant: keywords %v repeats %d, want Flashback once", g, rep[bit])
	}
	sc.s.grantFB(sc.k, o)
	if g, rep := sc.s.FeatGrantedOffRep(o); g != kop.KwFlashback || rep[bit] != 1 {
		t.Fatalf("two grants: keywords %v repeats %d, want Flashback twice", g, rep[bit])
	}
	sc.s.grantFB(sc.k, o)
	if g, rep := sc.s.FeatGrantedOffRep(o); g != kop.KwFlashback || rep[bit] != 2 {
		t.Fatalf("three grants: keywords %v repeats %d, want Flashback three times", g, rep[bit])
	}
	sc.s.grantFB(sc.k, o) // saturates at three grants
	if g, rep := sc.s.FeatGrantedOffRep(o); g != kop.KwFlashback || rep[bit] != 2 {
		t.Fatalf("four grants: keywords %v repeats %d, want Flashback three times (saturated)", g, rep[bit])
	}
	sc.s.clearFB(sc.k, o)
	if g, rep := sc.s.FeatGrantedOffRep(o); g != 0 || rep[bit] != 0 {
		t.Fatalf("cleared: keywords %v repeats %d, want none", g, rep[bit])
	}
	sc.s.setFBGrants(sc.k, o, 3) // a cast undone restores all three grants
	if g, rep := sc.s.FeatGrantedOffRep(o); g != kop.KwFlashback || rep[bit] != 2 {
		t.Fatalf("restored: keywords %v repeats %d, want Flashback three times", g, rep[bit])
	}
}

// TestCleanupMarksFlashbackGrants: a granted flashback is part of a
// graveyard card's view although no object field names it, so the cleanup
// that ends the grants must mark each card changed. Otherwise a per-object
// view cache (dirac/feat) keeps the stale Flashback keyword.
func TestCleanupMarksFlashbackGrants(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Inspiration from Beyond", Count: 1}})
	o := sc.obj(id["Inspiration from Beyond"], 0)
	sc.s.place(o, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, o)
	sc.s.grantFB(sc.k, o)
	sc.s.grantFB(sc.k, o)
	sc.s.SetEpoch(sc.s.Epoch() + 1) // clear the marks the place left
	at := sc.s.Touches()
	sc.s.cleanupGen(sc.k)
	if g, _ := sc.s.FeatGrantedOffRep(o); g != 0 {
		t.Fatalf("cleanup kept the grant: %v", g)
	}
	if !sc.s.Changed(o) || !sc.s.ChangedSince(o, at) {
		t.Fatal("cleanupGen ended the flashback grant without marking the card changed: a cached card view stays stale")
	}
}

// TestCastFlagsMarksFlashbackGrant: beginning a cast ends a granted
// flashback before the card leaves the graveyard (castFlagsGen), while the
// pre-push build can still encode the card. The grant is part of the
// graveyard card's view, so the clear must mark the card changed, or a
// per-object view cache keeps the stale Flashback.
func TestCastFlagsMarksFlashbackGrant(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Inspiration from Beyond", Count: 1}})
	o := sc.obj(id["Inspiration from Beyond"], 0)
	sc.s.place(o, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, o)
	sc.s.SetEpoch(sc.s.Epoch() + 1) // clear the marks the setup left
	at := sc.s.Touches()
	sc.s.castFlagsGen(sc.k, o, 0)
	if g := sc.s.fbGrants(sc.k, o); g != 0 {
		t.Fatalf("castFlagsGen kept the grant: %d", g)
	}
	if !sc.s.Changed(o) || !sc.s.ChangedSince(o, at) {
		t.Fatal("castFlagsGen ended the flashback grant without marking the card changed: a cached card view stays stale")
	}
}

// TestCastFlagsKeepsHeldCastNoProgress: the invalidation mark castFlagsGen
// adds must not read as progress to gorge's hold-out rule. Beginning the
// same card's cast again after an abort with nothing else changed is still
// no progress, so the second undo holds the card out (castHeldOut).
func TestCastFlagsKeepsHeldCastNoProgress(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Inspiration from Beyond", Count: 1}})
	o := sc.obj(id["Inspiration from Beyond"], 0)
	sc.s.place(o, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, o) // the abort restored the grant before the next begin
	sc.s.heldObj, sc.s.heldN = o, 1
	sc.s.heldTurn, sc.s.heldStep, sc.s.heldTouch = uint16(sc.s.turn), sc.s.step, sc.s.Touches()
	sc.s.castFlagsGen(sc.k, o, 0)
	if !sc.s.noProgress() {
		t.Fatal("castFlagsGen's invalidation mark counted as progress: a held cast's second undo would not be held out")
	}
	sc.s.heldN = 2
	if !sc.s.castHeldOut(o) {
		t.Fatal("a second undo with no progress did not hold the card out")
	}
}
