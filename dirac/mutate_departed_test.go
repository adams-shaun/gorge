package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// An ability whose source was a mutated permanent that has left the
// battlefield keeps the keywords a card beneath the pile's top printed
// (CR 702.140e, 113.7a, 608.2h): gorge derives infect and wither at
// departure over the whole pile, so the resolving ability still gives poison
// counters or puts -1/-1 counters.

// departedPileScen builds a pile with under beneath the keyword-less bird,
// puts one of the pile's activated abilities on the stack, removes the pile
// (its departure snapshot is taken) and starts the ability resolving. check
// runs on the live pile: it must name the keyword the damage then asserts.
func departedPileScen(t *testing.T, under CardID, check func(sc *scen, b Obj)) *scen {
	t.Helper()
	tbl, c := underKwTable(false)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	b := underPile(t, sc, under, c.bird)
	if tbl := sc.k.t; tbl.Cards[c.bird].Infect || tbl.Cards[c.bird].Wither {
		t.Fatalf("precondition: the pile's top card prints infect or wither")
	}
	if !sc.k.gen.anyInfect {
		t.Fatal("precondition: the table is not an infect table")
	}
	check(sc, b)
	e := stackEntry{kind: seAbility, obj: b, srcVer: sc.s.objs[b].ver, card: c.bird, ctrl: 0}
	sc.s.sp = 1
	sc.s.stack[0] = e
	sc.s.removePerm(sc.k, sc.slot(b), kop.ZoneExile, false)
	if got := sc.s.objs[b].zone; got != kop.ZoneExile {
		t.Fatalf("precondition: the source is in zone %d, want exile", got)
	}
	if i := sc.s.srcDepartIdx(&e); i == 0 {
		t.Fatal("precondition: the departed source has no departure snapshot")
	}
	sc.s.sp = 0
	sc.s.res = resCtx{e: e, active: true}
	return sc
}

func TestMutateDepartedSourceUnderInfect(t *testing.T) {
	_, c := underKwTable(false)
	sc := departedPileScen(t, c.plague, func(sc *scen, b Obj) {
		if !sc.s.infectAt(sc.k, sc.slot(b)) {
			t.Fatal("precondition: the live pile has no infect")
		}
	})
	life := sc.s.pl[1].life
	if n := sc.s.damageTo(sc.k, TCode(PlayerTarget1), 2); n != 2 {
		t.Fatalf("damageTo = %d, want 2", n)
	}
	if got, now := sc.s.Poison(1), sc.s.pl[1].life; got != 2 || now != life {
		t.Fatalf("player: poison %d life %d, want 2 and %d: the departed pile's under card's infect", got, now, life)
	}
}

func TestMutateDepartedSourceUnderWither(t *testing.T) {
	_, c := underKwTable(false)
	sc := departedPileScen(t, c.rot, func(sc *scen, b Obj) {
		if !sc.s.witherAt(sc.k, sc.slot(b)) {
			t.Fatal("precondition: the live pile has no wither")
		}
	})
	blk := sc.perm(c.warden, 1)
	if n := sc.s.damageTo(sc.k, TCode(blk), 1); n != 1 {
		t.Fatalf("damageTo = %d, want 1", n)
	}
	if p := &sc.s.perms[sc.slot(blk)]; p.damage != 0 || p.ctrLow(int(kop.CtrM1M1)) != 1 {
		t.Fatalf("blocker: damage %d, -1/-1 counters %d, want 0 and 1: the departed pile's under card's wither",
			p.damage, p.ctrLow(int(kop.CtrM1M1)))
	}
}

// departedStaleScen is departedPileScen for a self trigger whose own snapshot
// is older than the departure: the trigger is queued while the pile still has
// the under card's keyword, the pile then loses all abilities and leaves. Its
// last known information is the departure one (CR 608.2h, 702.80b, 702.90d),
// which has neither keyword; bit names the one the queue-time snapshot holds.
func departedStaleScen(t *testing.T, under CardID, bit uint8) *scen {
	t.Helper()
	tbl, c := underKwTable(false)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	b := underPile(t, sc, under, c.bird)
	s := sc.s
	s.enqueueLKI(b, s.objs[b].ver, c.bird, 0, 0, b, 0, 0)
	if s.npend != 1 || s.pend[0].li == 0 || s.lk(s.pend[0].li).bits&bit == 0 {
		t.Fatalf("precondition: npend %d, the queue-time snapshot lacks bit %#x", s.npend, bit)
	}
	e := stackEntry{kind: seTrigger, obj: b, srcVer: s.objs[b].ver, card: c.bird, ctrl: 0, trigObj: b, li: s.pend[0].li}
	s.npend = 0
	s.sp = 1
	s.stack[0] = e
	s.perms[sc.slot(b)].ovr.flags |= ovLoseAll
	s.removePerm(sc.k, sc.slot(b), kop.ZoneExile, false)
	if got := s.objs[b].zone; got != kop.ZoneExile {
		t.Fatalf("precondition: the source is in zone %d, want exile", got)
	}
	e = s.stack[0]
	if l := s.srcDepartRec(&e); l == nil || l.bits&bit != 0 {
		t.Fatalf("precondition: departure snapshot %+v, want one without bit %#x", l, bit)
	}
	if e.li == 0 || s.lk(e.li).obj != b || s.lk(e.li).bits&bit == 0 {
		t.Fatalf("precondition: the entry's snapshot %d is no longer the older one with bit %#x", e.li, bit)
	}
	s.sp = 0
	s.res = resCtx{e: e, active: true}
	return sc
}

func TestMutateDepartedSourceStaleInfect(t *testing.T) {
	_, c := underKwTable(false)
	sc := departedStaleScen(t, c.plague, lkiInfect)
	life := sc.s.pl[1].life
	if n := sc.s.damageTo(sc.k, TCode(PlayerTarget1), 2); n != 2 {
		t.Fatalf("damageTo = %d, want 2", n)
	}
	if got, now := sc.s.Poison(1), sc.s.pl[1].life; got != 0 || now != life-2 {
		t.Fatalf("player: poison %d life %d, want 0 and %d: the pile lost infect before it left", got, now, life-2)
	}
}

func TestMutateDepartedSourceStaleWither(t *testing.T) {
	_, c := underKwTable(false)
	sc := departedStaleScen(t, c.rot, lkiWither)
	blk := sc.perm(c.warden, 1)
	if n := sc.s.damageTo(sc.k, TCode(blk), 1); n != 1 {
		t.Fatalf("damageTo = %d, want 1", n)
	}
	if p := &sc.s.perms[sc.slot(blk)]; p.damage != 1 || p.ctrLow(int(kop.CtrM1M1)) != 0 {
		t.Fatalf("blocker: damage %d, -1/-1 counters %d, want 1 and 0: the pile lost wither before it left",
			p.damage, p.ctrLow(int(kop.CtrM1M1)))
	}
}

// A reflexive bite from a captured pile that left the battlefield deals with
// the infect a card beneath the top printed (reflexBiteGone reads the
// capture's snapshot, not the top card).
func TestMutateDepartedReflexBiteUnderInfect(t *testing.T) {
	tbl, c := underKwTable(false)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	b := underPile(t, sc, c.plague, c.bird)
	s, k := sc.s, sc.k
	if k.t.Cards[s.objs[b].card].Infect {
		t.Fatal("precondition: the pile's top card prints infect")
	}
	blk := sc.perm(c.warden, 1)
	n := s.power(k, sc.slot(b))
	if n <= 0 {
		t.Fatalf("precondition: the pile's power is %d, want a positive bite", n)
	}
	s.res = resCtx{e: stackEntry{kind: seTrigger, obj: b, card: c.bird, ctrl: 0,
		trigObj: b, trigAmt: int16(s.objs[b].ver)}, active: true}
	s.res.e.tgt[0], s.res.e.tver[0] = TCode(blk), s.objs[blk].ver
	s.res.e.li = s.snapLKI(b)
	if s.res.e.li == 0 || s.lk(s.res.e.li).bits&lkiInfect == 0 {
		t.Fatal("precondition: the capture's snapshot has no infect")
	}
	s.removePerm(k, sc.slot(b), kop.ZoneExile, false)
	if !s.reflexStale() {
		t.Fatal("precondition: the captured pile is still the same object")
	}
	s.exec(k, &kop.Instr{Op: kop.OpBite, Obj: kop.RefDelayedRemembered, C: int16(kop.RefTarget0)})
	if p := &s.perms[sc.slot(blk)]; p.damage != 0 || p.ctrLow(int(kop.CtrM1M1)) != n {
		t.Fatalf("blocker: damage %d, -1/-1 counters %d, want 0 and %d: the departed pile's under card's infect",
			p.damage, p.ctrLow(int(kop.CtrM1M1)), n)
	}
}
