package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tailDivRun casts Run Away Together on the Coyote (p0, holding the exiled
// Sentinel under its link) and Valley Mightcaller (p1); without both the
// Coyote alone is bounced. It returns the triggers waiting after the spell resolves.
func tailDivRun(t *testing.T, both bool) (pend int, sentinelPend, mightPend int) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Driftgloom Coyote", Count: 2}, {Name: "Run Away Together", Count: 2}, {Name: "Stickytongue Sentinel", Count: 2},
		{Name: "Valley Mightcaller", Count: 2}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4}}
	sc, id := newBLBScen(t, deck)
	s := sc.s
	coy := sc.perm(id["Driftgloom Coyote"], 0)
	might := sc.perm(id["Valley Mightcaller"], 1)
	sen := sc.obj(id["Stickytongue Sentinel"], 1)
	s.place(sen, kop.ZoneExile, false)
	s.g.links[0].src, s.g.links[0].sver, s.g.links[0].obj = coy, s.objs[coy].ver, sen
	s.g.nlinks = 1
	if both {
		sc.perm(id["Island"], 0)
		sc.perm(id["Island"], 0)
		rat := sc.hand(id["Run Away Together"], 0)
		sc.castCard(t, rat)
		sc.do(t, AkTarget, scAny, TCode(coy))
		sc.do(t, AkTarget, scAny, TCode(might))
		for i := 0; i < 20 && s.objs[rat].zone == kop.ZoneStack; i++ {
			d := s.Step()
			if d.Kind == DkPriority {
				s.Apply(MakeAction(AkPass, 0, 0, 0))
			} else {
				s.Apply(d.Options[0])
			}
		}
	} else {
		s.leave(sc.k, sc.slot(coy), kop.ZoneHand, false) // a lone move: no departure frame
	}
	if s.objs[sen].zone != kop.ZoneBattlefield || s.objs[coy].zone != kop.ZoneHand {
		t.Fatalf("sentinel zone %d, coyote zone %d: the bounce did not run", s.objs[sen].zone, s.objs[coy].zone)
	}
	if both && s.objs[might].zone != kop.ZoneHand {
		t.Fatalf("Mightcaller zone %d, want hand", s.objs[might].zone)
	}
	count := func(o Obj) {
		for i := 0; i < s.NumPending(); i++ {
			if src, _, _, _, _ := s.PendingTrigger(i); src == o {
				if o == sen {
					sentinelPend++
				} else {
					mightPend++
				}
			}
		}
	}
	count(sen)
	count(might)
	return s.NumPending(), sentinelPend, mightPend
}

// TestLinkReturnInDepartureBatchHeld: the until-leaves return comes after one
// ChangeZone's departure batch (CR 610.3, 610.3d), so Valley Mightcaller, bounced
// in the same batch, has gone and does not trigger for the returning Frog; alone
// it does.
func TestLinkReturnInDepartureBatchHeld(t *testing.T) {
	n, sen, mc := tailDivRun(t, true)
	if n != 1 || sen != 1 || mc != 0 {
		t.Errorf("both bounced: pending %d (sentinel %d, mightcaller %d), want 1 / 1 / 0", n, sen, mc)
	}
	n, sen, mc = tailDivRun(t, false)
	if n != 2 || sen != 1 || mc != 1 {
		t.Errorf("coyote alone: pending %d (sentinel %d, mightcaller %d), want 2 / 1 / 1", n, sen, mc)
	}
}

// exiledUntilLeaves puts a card c of owner's into exile until src leaves.
func exiledUntilLeaves(sc *scen, src Obj, c CardID, owner uint8) Obj {
	s := sc.s
	o := sc.obj(c, owner)
	s.place(o, kop.ZoneExile, false)
	l := &s.g.links[s.g.nlinks]
	l.src, l.sver, l.obj = src, s.objs[src].ver, o
	s.g.nlinks++
	return o
}

// TestLinkReturnFindsTheBoardAfterTheBatch: the batch is one event, so a
// returning Aura sees the board without everything that left in it (CR 610.3): it
// stays in exile with nothing to enchant (CR 303.4g), and otherwise enters on the
// survivor with no ask over a creature that left beside its source.
func TestLinkReturnFindsTheBoardAfterTheBatch(t *testing.T) {
	deck := []cardset.Entry{{Name: "Driftgloom Coyote", Count: 2}, {Name: "War Squeak", Count: 2}, {Name: "Stickytongue Sentinel", Count: 2},
		{Name: "Valley Mightcaller", Count: 2}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4}}
	for _, survivor := range []bool{false, true} {
		sc, id := newBLBScen(t, deck)
		s := sc.s
		coy := sc.perm(id["Driftgloom Coyote"], 0)
		might := sc.perm(id["Valley Mightcaller"], 1)
		aura := exiledUntilLeaves(sc, coy, id["War Squeak"], 1)
		var m slotMask
		m.set(sc.slot(coy))
		m.set(sc.slot(might))
		var kept Obj
		if survivor {
			kept = sc.perm(id["Stickytongue Sentinel"], 1)
		}
		s.leaveAll(sc.k, m, kop.ZoneGraveyard)
		if s.g.nask != 0 {
			t.Errorf("survivor %v: %d asks, want none", survivor, s.g.nask)
		}
		if !survivor {
			if z := s.objs[aura].zone; z != kop.ZoneExile {
				t.Errorf("no survivor: the Aura is in zone %d, want exile", z)
			}
			continue
		}
		if z := s.objs[aura].zone; z != kop.ZoneBattlefield {
			t.Fatalf("survivor: the Aura is in zone %d, want the battlefield", z)
		}
		if got := s.perms[sc.slot(aura)].attached; got != sc.slot(kept) {
			t.Errorf("survivor: the Aura is on slot %d, want %d", got, sc.slot(kept))
		}
	}
}

// TestLinkReturnsInDepartureOrder: the cards come back source by source in the
// order the sources left, not the order the links were made.
func TestLinkReturnsInDepartureOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Driftgloom Coyote", Count: 2}, {Name: "Stickytongue Sentinel", Count: 2},
		{Name: "Valley Mightcaller", Count: 2}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4}}
	sc, id := newBLBScen(t, deck)
	s := sc.s
	first := sc.perm(id["Driftgloom Coyote"], 0)
	second := sc.perm(id["Driftgloom Coyote"], 1)
	late := exiledUntilLeaves(sc, second, id["Stickytongue Sentinel"], 1)
	early := exiledUntilLeaves(sc, first, id["Valley Mightcaller"], 0)
	var m slotMask
	m.set(sc.slot(first))
	m.set(sc.slot(second))
	s.leaveAll(sc.k, m, kop.ZoneGraveyard)
	if s.objs[early].zone != kop.ZoneBattlefield || s.objs[late].zone != kop.ZoneBattlefield {
		t.Fatalf("zones %d and %d, want both on the battlefield", s.objs[early].zone, s.objs[late].zone)
	}
	if a, b := s.perms[sc.slot(early)].ts, s.perms[sc.slot(late)].ts; a >= b {
		t.Errorf("entry stamps %d and %d: the first source's card should enter first", a, b)
	}
}

// TestLinkReturnsAreTheirOwnBatch: the returns are a fresh batch inside the
// frame's close (CR 610.3d). Valley Questcaller's "one or more" line, already
// queued in the batch the frame sits in, queues once more for the two Rabbits
// returning together, and the enclosing batch is intact afterwards.
func TestLinkReturnsAreTheirOwnBatch(t *testing.T) {
	deck := []cardset.Entry{{Name: "Valley Questcaller", Count: 1}, {Name: "Driftgloom Coyote", Count: 2},
		{Name: "Seasoned Warrenguard", Count: 4}, {Name: "Plains", Count: 8}}
	sc, id := newBLBScen(t, deck)
	s := sc.s
	qc, wg := id["Valley Questcaller"], id["Seasoned Warrenguard"]
	q := sc.perm(qc, 0)
	ab := batchAb(t, sc, qc, kop.Tf2Batch)
	first := sc.perm(id["Driftgloom Coyote"], 1)
	second := sc.perm(id["Driftgloom Coyote"], 1)
	a := exiledUntilLeaves(sc, first, wg, 0)
	b := exiledUntilLeaves(sc, second, wg, 0)

	old := s.batchOpen()
	hr := s.holdOpen(sc.k)
	o := sc.hand(wg, 0)
	s.detach(o)
	s.enter(sc.k, o, 0, false, kop.ZoneNone)
	s.leave(sc.k, sc.slot(first), kop.ZoneHand, false)
	s.leave(sc.k, sc.slot(second), kop.ZoneHand, false)
	if n := batchPend(sc, q, ab); n != 1 {
		t.Fatalf("before the close: %d Questcaller triggers, want 1", n)
	}
	if s.objs[a].zone != kop.ZoneExile || s.objs[b].zone != kop.ZoneExile {
		t.Fatalf("zones %d and %d before the close, want both in exile", s.objs[a].zone, s.objs[b].zone)
	}
	open := s.g.batchLatch & batchOpenMask
	s.holdClose(sc.k, hr)
	if s.objs[a].zone != kop.ZoneBattlefield || s.objs[b].zone != kop.ZoneBattlefield {
		t.Fatalf("zones %d and %d after the close, want both on the battlefield", s.objs[a].zone, s.objs[b].zone)
	}
	if n := batchPend(sc, q, ab); n != 2 {
		t.Errorf("after the close: %d Questcaller triggers, want 2", n)
	}
	if got := s.g.batchLatch & batchOpenMask; got != open {
		t.Errorf("the enclosing batch reads %d after the close, want %d", got, open)
	}
	s.batchClose(old)
	if s.g.batchLatch != 0 {
		t.Errorf("batch state left set: %#x", s.g.batchLatch)
	}
}

// TestDepartureFrameLeavesShieldsAlone: in a link-return table a permanent
// with two regeneration shields keeps both across a departure frame and gets
// no spurious enters check (the held entrants are a slot mask, not a perm
// flag that aliases the shield bits).
func TestDepartureFrameLeavesShieldsAlone(t *testing.T) {
	deck := []cardset.Entry{{Name: "Driftgloom Coyote", Count: 2}, {Name: "Stickytongue Sentinel", Count: 2},
		{Name: "Valley Mightcaller", Count: 2}, {Name: "Island", Count: 4}, {Name: "Plains", Count: 4}}
	sc, id := newBLBScen(t, deck)
	s := sc.s
	if !sc.k.gen.anyLinkRet {
		t.Fatal("table has no until-leaves exiler")
	}
	sen := sc.perm(id["Stickytongue Sentinel"], 0)
	might := sc.perm(id["Valley Mightcaller"], 1)
	sl := sc.slot(sen)
	s.setRegenShields(sl, 2)
	var m slotMask
	m.set(sc.slot(might))
	s.leaveAll(sc.k, m, kop.ZoneHand)
	if got := s.regenShieldsAt(sl); got != 2 {
		t.Errorf("shields %d, want 2", got)
	}
	if n := s.NumPending(); n != 0 {
		t.Errorf("pending %d, want 0", n)
	}
}
