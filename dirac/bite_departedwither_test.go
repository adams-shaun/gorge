package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBiteDepartedWitherPutsCounters: a reflexive bite from a captured pile
// that left the battlefield, with a wither card beneath its top, deals its
// damage to a creature as -1/-1 counters (CR 702.80b, 113.7a): the capture's
// snapshot carries the pile's wither (lkiWither), which dmgSrcObject's
// departed-source path must honour as it honours infect.
func TestBiteDepartedWitherPutsCounters(t *testing.T) {
	tbl, c := underKwTable(false)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	b := underPile(t, sc, c.rot, c.bird)
	s, k := sc.s, sc.k
	if cd := &k.t.Cards[s.objs[b].card]; cd.Wither || cd.Infect {
		t.Fatal("precondition: the pile's top card prints wither or infect")
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
	if s.res.e.li == 0 || s.lk(s.res.e.li).bits&lkiWither == 0 || s.lk(s.res.e.li).bits&lkiInfect != 0 {
		t.Fatal("precondition: the capture's snapshot has no wither (or has infect)")
	}
	s.removePerm(k, sc.slot(b), kop.ZoneExile, false)
	if !s.reflexStale() {
		t.Fatal("precondition: the captured pile is still the same object")
	}
	s.exec(k, &kop.Instr{Op: kop.OpBite, Obj: kop.RefDelayedRemembered, C: int16(kop.RefTarget0)})
	if p := &s.perms[sc.slot(blk)]; p.damage != 0 || p.ctrLow(int(kop.CtrM1M1)) != n {
		t.Fatalf("blocker: damage %d, -1/-1 counters %d, want 0 and %d: the departed pile's under card's wither",
			p.damage, p.ctrLow(int(kop.CtrM1M1)), n)
	}
}
