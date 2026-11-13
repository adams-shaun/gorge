package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// parkedReturnFrame builds a resolution with a bare test sorcery (an empty
// program) and an Aura whose source has just been sacrificed: the departure
// frame parks the Aura's host ask and keeps its entry batch open (holdClose).
func parkedReturnFrame(t *testing.T) (*scen, CardID, Obj, Obj, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Valley Questcaller", Count: 1}, {Name: "Driftgloom Coyote", Count: 2},
		{Name: "Seasoned Warrenguard", Count: 1}, {Name: "War Squeak", Count: 1},
		{Name: "Valley Mightcaller", Count: 1}, {Name: "Plains", Count: 8}}
	var spell CardID
	sc, id := newBLBScenTweak(t, deck, func(tbl *kop.Table, _ map[string]CardID) {
		spell = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Parked Return", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell}}})
	})
	s := sc.s
	qc := id["Valley Questcaller"]
	q := sc.perm(qc, 0)
	sc.perm(id["Valley Mightcaller"], 1) // the returning Aura's second host
	auraSrc := sc.perm(id["Driftgloom Coyote"], 0)
	tailSrc := sc.perm(id["Driftgloom Coyote"], 1)
	aura := exiledUntilLeaves(sc, auraSrc, id["War Squeak"], 0)
	late := exiledUntilLeaves(sc, tailSrc, id["Seasoned Warrenguard"], 0)

	sp := sc.obj(spell, 0)
	s.objs[sp].zone = kop.ZoneStack
	s.sp = 1
	s.res = resCtx{e: stackEntry{kind: seSpell, obj: sp, card: spell, ctrl: 0, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve

	// The frame parks inside an answer's sacrifice, on the program's last
	// instruction (the empty sorcery's end): holdClose sets retHold and
	// leaves the entry batch latched for the answer.
	s.addSac(sc.slot(auraSrc))
	s.flushSac(sc.k)
	if s.g.pre.state != preReturn || s.g.pre.obj != aura {
		t.Fatalf("pre state %d obj %d, want the Aura's host ask parked", s.g.pre.state, s.g.pre.obj)
	}
	if !s.g.retHold || s.g.batchLatch&batchHeld == 0 {
		t.Fatalf("retHold %v latch %#x, want the parked frame open", s.g.retHold, s.g.batchLatch)
	}
	if s.objs[aura].zone != kop.ZoneExile {
		t.Fatalf("the Aura is in zone %d before the answer, want exile", s.objs[aura].zone)
	}
	if got := batchPend(sc, q, batchAb(t, sc, qc, kop.Tf2Batch)); got != 0 {
		t.Fatalf("%d Questcaller triggers before the answer, want 0", got)
	}
	return sc, qc, q, aura, tailSrc, late
}

// assertFrameClosed is the shared post-answer check: the Aura entered and the
// departure frame is gone, so a later until-leaves return is not held by it.
func assertFrameClosed(t *testing.T, sc *scen, q Obj, aura, tailSrc, late Obj) {
	t.Helper()
	s := sc.s
	if z := s.objs[aura].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the Aura is in zone %d after the answer, want the battlefield", z)
	}
	if s.g.retHold || s.g.nretHeld != 0 || s.g.held != 0 || s.g.batchLatch&(batchHeld|batchOpenMask) != 0 {
		t.Fatalf("after the answer retHold %v nretHeld %d held %#x latch %#x, want the frame closed",
			s.g.retHold, s.g.nretHeld, s.g.held, s.g.batchLatch)
	}
	var m slotMask
	m.set(sc.slot(tailSrc))
	s.leaveAll(sc.k, m, kop.ZoneGraveyard)
	if z := s.objs[late].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the later until-leaves card is in zone %d, want the battlefield: the frame leaked", z)
	}
}

// TestLinkReturnParkedInAnswerClosesFrame: a sacrifice answered through an
// answer arm parks its return's host ask on the program's last instruction;
// the ask is posed before the program advances (continueRes), and answering
// it closes the departure frame (CR 603.6a, 610.3d).
func TestLinkReturnParkedInAnswerClosesFrame(t *testing.T) {
	sc, _, q, aura, tailSrc, late := parkedReturnFrame(t)
	s := sc.s

	s.continueRes(sc.k, 0)
	if s.res.pc != 0 {
		t.Errorf("after continueRes pc %d, want the parked ask to wait before the program advances", s.res.pc)
	}
	if s.res.ask != raGenRetChoice || s.g.pre.obj != aura {
		t.Fatalf("after continueRes ask %d pre obj %d, want the parked Aura's ask", s.res.ask, s.g.pre.obj)
	}
	if !s.g.retHold || s.g.batchLatch&batchHeld == 0 {
		t.Fatalf("after continueRes retHold %v latch %#x, want the frame still open across the ask",
			s.g.retHold, s.g.batchLatch)
	}
	s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(q), 0, 0))
	assertFrameClosed(t, sc, q, aura, tailSrc, late)
}

// TestFinishResolveClosesParkedReturnFrame: the same parked return consumed by
// finishResolve (a return set aside outside the program loop) enters through
// enterLinkRet, drains the held returns and closes the frame.
func TestFinishResolveClosesParkedReturnFrame(t *testing.T) {
	sc, _, q, aura, tailSrc, late := parkedReturnFrame(t)
	sc.s.finishResolve(sc.k, false)
	assertFrameClosed(t, sc, q, aura, tailSrc, late)
}
