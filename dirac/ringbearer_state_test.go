package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newRingBearerScen is a two-seat scenario whose table tempts the Ring (the
// thering fixture's Birthday Escape) beside two Grizzly Bears, so a
// designation can be made and its object blinked or stolen.
func newRingBearerScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	return newSetScenTweak(t, "thering", []cardset.Entry{
		{Name: "Birthday Escape", Count: 1},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 8},
	}, nil)
}

// ringDefaultNow is ringDefault over the seat's live eligible creatures.
func ringDefaultNow(s *State, k *prep, p uint8) Obj {
	var buf [MaxPerm]Slot
	n := s.ringEligible(k, p, &buf)
	return s.ringDefault(k, p, buf[:n])
}

// TestRingBearerBlinkClearsDesignation: a Ring-bearer that leaves and
// returns is a new object (CR 400.7) and the designation is gone (gorge
// clearRingBearers on every battlefield leave). The deterministic default
// falls back to the first eligible creature in entry order, which is the
// other Bear after the return.
func TestRingBearerBlinkClearsDesignation(t *testing.T) {
	sc, id := newRingBearerScen(t)
	a := sc.perm(id["Grizzly Bears"], 0)
	b := sc.perm(id["Grizzly Bears"], 0)
	sc.s.ringFold(sc.k, 0, a)
	if got := ringDefaultNow(sc.s, sc.k, 0); got != a {
		t.Fatalf("default before the blink = %d, want the designated Bear %d", got, a)
	}

	sl := sc.slot(a)
	sc.s.leave(sc.k, sl, kop.ZoneExile, false)
	if sc.s.objs[a].zone != kop.ZoneExile {
		t.Fatalf("the Bear did not leave to exile: zone %v", sc.s.objs[a].zone)
	}
	sc.s.enter(sc.k, a, 0, false, kop.ZoneExile)
	if !sc.onBF(a) {
		t.Fatalf("the Bear did not return to the battlefield: zone %v", sc.s.objs[a].zone)
	}
	nsl := sc.slot(a)
	if sc.s.ringLegend(sc.k, nsl) {
		t.Fatal("the returned Bear is still the emblem's Ring-bearer")
	}
	if sc.s.ringTySlots(sc.k).has(nsl) {
		t.Fatal("the returned Bear is still in the emblem's type set")
	}
	if got := ringDefaultNow(sc.s, sc.k, 0); got != b {
		t.Fatalf("default after the blink = %d, want the other Bear %d (the returned Bear is a new object and rejoined the end of the zone order)", got, b)
	}
}

// TestRingBearerControlChangeClearsDesignation: a control change ends the
// designation even when control later returns (gorge foldControlChange clears
// on the change). The returned Bear rejoins the end of the controller's
// battlefield order, so the default is the other Bear.
func TestRingBearerControlChangeClearsDesignation(t *testing.T) {
	sc, id := newRingBearerScen(t)
	a := sc.perm(id["Grizzly Bears"], 0)
	b := sc.perm(id["Grizzly Bears"], 0)
	sc.s.ringFold(sc.k, 0, a)
	if got := ringDefaultNow(sc.s, sc.k, 0); got != a {
		t.Fatalf("default before the control change = %d, want the designated Bear %d", got, a)
	}

	sl := sc.slot(a)
	if !sc.s.gainControl(sl, 1, 0, kop.DurEOT, NoObj, 0, false) {
		t.Fatal("gainControl to seat 1 failed")
	}
	if !sc.s.gainControl(sl, 0, 0, kop.DurEOT, NoObj, 0, false) {
		t.Fatal("gainControl back to seat 0 failed")
	}
	if sc.s.perms[sl].ctrl != 0 {
		t.Fatalf("the Bear's controller is %d after the round trip, want 0", sc.s.perms[sl].ctrl)
	}
	if sc.s.ringLegend(sc.k, sl) {
		t.Fatal("the returned Bear is still the emblem's Ring-bearer")
	}
	if sc.s.ringTySlots(sc.k).has(sl) {
		t.Fatal("the returned Bear is still in the emblem's type set")
	}
	if got := ringDefaultNow(sc.s, sc.k, 0); got != b {
		t.Fatalf("default after the control round trip = %d, want the other Bear %d (the designation ended at the control change)", got, b)
	}
}

// TestRingBearerCardPredicateOffBattlefield: predCardCold must answer the
// Ring-bearer word off the battlefield (nothing there is a Ring-bearer, so a
// negated word is true) instead of panicking. A granted protection with
// Card.IsRingbearer is the path that reaches it.
func TestRingBearerCardPredicateOffBattlefield(t *testing.T) {
	sc, id := newRingBearerScen(t)
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredRingBearer, PredN: 0})
	notBearer := kop.FilterID(len(sc.k.t.Filters) - 1)
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredRingBearer, PredN: 1})
	bearer := kop.FilterID(len(sc.k.t.Filters) - 1)
	c := id["Grizzly Bears"]
	if !sc.s.matchCard(sc.k, notBearer, c, 0, 0) {
		t.Fatal("!IsRingbearer is false for a card off the battlefield")
	}
	if sc.s.matchCard(sc.k, bearer, c, 0, 0) {
		t.Fatal("IsRingbearer is true for a card off the battlefield")
	}
}

// TestRingPromiseControllerIsEmblemOwner: the level-3 delayed sacrifice is
// controlled by the emblem's owner (gorge effRingBlockerSacrifice schedules
// with c.Controller), not by the blocker's controller; the sacrifice itself
// still runs under the permanent's controller when it resolves.
func TestRingPromiseControllerIsEmblemOwner(t *testing.T) {
	sc, id := newRingBearerScen(t)
	b := sc.perm(id["Grizzly Bears"], 1)
	e := &stackEntry{trigObj: b, srcVer: sc.s.objs[b].ver, ctrl: 0, trigAmt: ringBlockAmt}
	sc.s.ringPromiseReg(sc.k, e)
	promises := sc.s.ringPromiseCold()
	n := sc.s.ringPromiseN()
	if len(promises) == 0 || n == nil || *n != 1 {
		t.Fatalf("registered %d promises (n %v), want 1", len(promises), n)
	}
	if promises[0].ctrl != 0 {
		t.Fatalf("the promise is controlled by seat %d, want the emblem owner 0", promises[0].ctrl)
	}
	sc.s.fireRingPromises(sc.k)
	if sc.s.npend != 1 {
		t.Fatalf("queued %d pending triggers, want 1", sc.s.npend)
	}
	if got := sc.s.pend[0].ctrl; got != 0 {
		t.Fatalf("the delayed sacrifice's controller is seat %d, want the emblem owner 0", got)
	}
}
