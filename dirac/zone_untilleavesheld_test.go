package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLinkReturnsParkedHostAskKeepsBatch: a departure batch whose returns park
// on a returning Aura's host ask keeps the batch's entry frame open across the
// answer (CR 603.6a, 610.3d). The Rabbits returned after the Aura enter with it
// as one batch, so Valley Questcaller's "one or more" line queues once rather
// than once per card, and every held enters check fires after the last entry.
func TestLinkReturnsParkedHostAskKeepsBatch(t *testing.T) {
	deck := []cardset.Entry{{Name: "Valley Questcaller", Count: 1}, {Name: "Driftgloom Coyote", Count: 2},
		{Name: "Seasoned Warrenguard", Count: 2}, {Name: "War Squeak", Count: 1},
		{Name: "Valley Mightcaller", Count: 1}, {Name: "Plains", Count: 8}}
	var spell CardID
	sc, id := newBLBScenTweak(t, deck, func(tbl *kop.Table, _ map[string]CardID) {
		spell = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Batch Return", Types: kop.TSorcery,
			Abilities: []kop.Ability{{Kind: kop.AbSpell}}})
	})
	s := sc.s
	qc, wg := id["Valley Questcaller"], id["Seasoned Warrenguard"]
	q := sc.perm(qc, 0)
	ab := batchAb(t, sc, qc, kop.Tf2Batch)
	sc.perm(id["Valley Mightcaller"], 1) // the returning Aura's second host
	auraSrc := sc.perm(id["Driftgloom Coyote"], 1)
	tailSrc := sc.perm(id["Driftgloom Coyote"], 1)
	aura := exiledUntilLeaves(sc, auraSrc, id["War Squeak"], 0)
	a := exiledUntilLeaves(sc, tailSrc, wg, 0)
	b := exiledUntilLeaves(sc, tailSrc, wg, 0)

	sp := sc.obj(spell, 0)
	s.objs[sp].zone = kop.ZoneStack
	s.sp = 1
	s.res = resCtx{e: stackEntry{kind: seSpell, obj: sp, card: spell, ctrl: 0, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj}
	s.res.e.tgt = noTargets
	s.ctl = ctlResolve
	s.npend = 0

	var m slotMask
	m.set(sc.slot(auraSrc))
	m.set(sc.slot(tailSrc))
	s.leaveAll(sc.k, m, kop.ZoneGraveyard)

	if s.g.pre.state != preReturn || s.g.pre.obj != aura {
		t.Fatalf("pre state %d obj %d, want the Aura's host ask parked before the rest returns", s.g.pre.state, s.g.pre.obj)
	}
	if s.g.nretHeld != 1 {
		t.Fatalf("nretHeld %d, want the source of the two Rabbits held behind the Aura's ask", s.g.nretHeld)
	}
	if !s.g.retHold || s.g.batchLatch&batchHeld == 0 {
		t.Errorf("retHold %v latch %#x, want the entry frame open across the ask (the behaviour below needs it)",
			s.g.retHold, s.g.batchLatch)
	}
	for _, o := range []Obj{aura, a, b} {
		if s.objs[o].zone != kop.ZoneExile {
			t.Fatalf("card %d zone %d before the answer, want exile", o, s.objs[o].zone)
		}
	}
	if n := batchPend(sc, q, ab); n != 0 {
		t.Fatalf("%d Questcaller triggers before the answer, want 0", n)
	}

	// runProg poses the parked ask after the op that made the departures returns.
	s.res.ask, s.res.askP = raGenRetChoice, s.objs[aura].owner()
	s.applyResolveGen(sc.k, MakeAction(AkChoose, TCode(q), 0, 0))

	for _, o := range []Obj{aura, a, b} {
		if s.objs[o].zone != kop.ZoneBattlefield {
			t.Fatalf("card %d zone %d after the answer, want the battlefield", o, s.objs[o].zone)
		}
	}
	if s.perms[sc.slot(aura)].attached != sc.slot(q) {
		t.Fatalf("the Aura entered attached to %d, want %d", s.perms[sc.slot(aura)].attached, sc.slot(q))
	}
	if n := batchPend(sc, q, ab); n != 1 {
		t.Fatalf("%d Questcaller triggers after the answer, want 1: the Aura and both Rabbits are one entry batch", n)
	}
	if s.g.retHold || s.g.nretHeld != 0 || s.g.held != 0 || s.g.batchLatch&(batchHeld|batchOpenMask) != 0 {
		t.Fatalf("after the answer retHold %v nretHeld %d held %#x latch %#x, want the frame closed",
			s.g.retHold, s.g.nretHeld, s.g.held, s.g.batchLatch)
	}
}
