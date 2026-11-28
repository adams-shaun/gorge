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
