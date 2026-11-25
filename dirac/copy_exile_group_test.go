package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A CopyPermanent token with AtEOT$ Exile over several copies of one
// resolution is one delayed triggered ability naming them all (CR 603.7a/b;
// gorge scheduleAtEOTGroup), so only the first copy's record is a head due at
// the end step; every later copy is a follower (trig_ateotexilegroup.go) and
// the end step queues one trigger.
func TestCopyPermanentExileAtEndGroups(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Carrot Cake", Count: 1}, {Name: "Plains", Count: 8}})
	if sc.k.gen == nil {
		t.Fatal("BLB table has no generated extension; the copy entry needs one")
	}
	c := CardID(0)
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Types&kop.TCreature != 0 {
			c = CardID(i)
			break
		}
	}
	if c == 0 {
		t.Fatal("no creature card in the BLB table")
	}
	src := sc.s.newToken(c, 0)
	sc.s.res = resCtx{e: stackEntry{kind: seSpell, ctrl: 0, obj: src}, active: true}
	in := kop.Instr{Op: kop.OpCopyAsTokenEx, B: kop.Const(kop.CpExileAtEnd)}

	n0 := sc.s.ndly
	first := sc.s.newToken(c, 0)
	sc.s.copyTokenEnterFrom(sc.k, &in, first, NoSlot, NoSlot, nil)
	sc.s.res.flags |= rfCopyGrp
	second := sc.s.newToken(c, 0)
	sc.s.copyTokenEnterFrom(sc.k, &in, second, NoSlot, NoSlot, nil)

	if got := sc.s.ndly - n0; got != 2 {
		t.Fatalf("delayed records %d -> %d, want the head and one follower", n0, sc.s.ndly)
	}
	head, follower := sc.s.dly[n0], sc.s.dly[n0+1]
	if head.act != uint8(kop.OpExile) || head.when != 0 || head.obj != first || head.creator != src {
		t.Fatalf("head record = %+v, want the tracked next-end-step exile of %d sourced on %d", head, first, src)
	}
	if follower.act != uint8(kop.OpExile) || follower.when != dlyGrpFollow || follower.creator != first || follower.obj != second {
		t.Fatalf("follower record = %+v, want a follower of %d for %d", follower, first, second)
	}

	np := sc.s.npend
	sc.s.fireDelayed(sc.k)
	if got := sc.s.npend - np; got != 1 {
		t.Fatalf("the end step queued %d exile triggers, want one group trigger", got)
	}
}
