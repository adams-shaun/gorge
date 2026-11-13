package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// batchAb is the index of card c's first triggered ability with flags2 f.
func batchAb(t *testing.T, sc *scen, c CardID, f kop.TrigFlags2) uint8 {
	t.Helper()
	for j, a := range sc.k.t.Cards[c].Abilities {
		if a.Kind == kop.AbTriggered && a.Trig.Flags2&f == f {
			return uint8(j)
		}
	}
	t.Fatalf("%s has no trigger with flags2 %#x", sc.k.t.Cards[c].Name, f)
	return 0
}

// batchPend counts the pending triggers of object o's ability ab.
func batchPend(sc *scen, o Obj, ab uint8) int {
	n := 0
	for i := 0; i < int(sc.s.npend); i++ {
		if sc.s.pend[i].obj == o && sc.s.pend[i].ab == ab {
			n++
		}
	}
	return n
}

// TestChangesZoneAllEnterBatch: Valley Questcaller's "whenever one or
// more other Rabbits ... you control enter" (Mode$ ChangesZoneAll) queues
// once for an entry batch of two Rabbits (gorge's zone batch latch), once
// per entry outside a batch, and not at all for a batch with no member.
func TestChangesZoneAllEnterBatch(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Questcaller", Count: 1}, {Name: "Seasoned Warrenguard", Count: 4}, {Name: "Plains", Count: 8}})
	qc, wg, pl := id["Valley Questcaller"], id["Seasoned Warrenguard"], id["Plains"]
	q := sc.perm(qc, 0)
	ab := batchAb(t, sc, qc, kop.Tf2Batch)
	enterBatch := func(c CardID, n int) {
		var eb entBatch
		for i := 0; i < n; i++ {
			o := sc.hand(c, 0)
			sc.s.detach(o)
			sc.s.enterHeld(sc.k, &eb, o, 0, false, kop.ZoneNone)
		}
		sc.s.fireEntries(sc.k, &eb)
	}
	sc.s.npend = 0
	enterBatch(wg, 2)
	if n := batchPend(sc, q, ab); n != 1 {
		t.Fatalf("a batch of two Rabbits queued %d Questcaller triggers, want 1", n)
	}
	sc.s.npend = 0
	for i := 0; i < 2; i++ {
		o := sc.hand(wg, 0)
		sc.s.detach(o)
		sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
	}
	if n := batchPend(sc, q, ab); n != 2 {
		t.Fatalf("two separate Rabbit entries queued %d Questcaller triggers, want 2", n)
	}
	sc.s.npend = 0
	enterBatch(pl, 2)
	if n := batchPend(sc, q, ab); n != 0 {
		t.Fatalf("a batch of two Plains queued %d Questcaller triggers, want 0", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("batch state left open: %#x", sc.s.g.batchLatch)
	}
}

// TestChangesZoneAllLeaveBatch: Dour Port-Mage's "whenever one or more
// other creatures you control leave the battlefield without dying" queues
// once for a leaveAll of two of them to their owner's hand, not for a
// leaveAll to the graveyard (they died), and not for a batch of none.
func TestChangesZoneAllLeaveBatch(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Dour Port-Mage", Count: 1}, {Name: "Seasoned Warrenguard", Count: 4}, {Name: "Plains", Count: 8}})
	dm, wg := id["Dour Port-Mage"], id["Seasoned Warrenguard"]
	d := sc.perm(dm, 0)
	ab := batchAb(t, sc, dm, kop.Tf2Batch)
	two := func(p uint8) slotMask {
		var m slotMask
		m = m.with(sc.slot(sc.perm(wg, p)))
		return m.with(sc.slot(sc.perm(wg, p)))
	}
	sc.s.npend = 0
	sc.s.leaveAll(sc.k, two(0), kop.ZoneHand)
	if n := batchPend(sc, d, ab); n != 1 {
		t.Fatalf("two creatures bounced together queued %d Port-Mage triggers, want 1", n)
	}
	sc.s.npend = 0
	sc.s.leaveAll(sc.k, two(0), kop.ZoneGraveyard)
	if n := batchPend(sc, d, ab); n != 0 {
		t.Fatalf("two creatures dying queued %d Port-Mage triggers, want 0", n)
	}
	sc.s.npend = 0
	sc.s.leaveAll(sc.k, two(1), kop.ZoneExile)
	if n := batchPend(sc, d, ab); n != 0 {
		t.Fatalf("the opponent's creatures leaving queued %d Port-Mage triggers, want 0", n)
	}
	o1, o2 := sc.perm(wg, 0), sc.perm(wg, 0)
	sc.s.npend = 0
	sc.s.leave(sc.k, sc.slot(o1), kop.ZoneExile, false)
	sc.s.leave(sc.k, sc.slot(o2), kop.ZoneHand, false)
	if n := batchPend(sc, d, ab); n != 2 {
		t.Fatalf("two separate leaves queued %d Port-Mage triggers, want 2", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("batch state left set: %#x", sc.s.g.batchLatch)
	}
}

// TestSacrificedOnce: Camellia's "whenever you sacrifice one or more
// Foods" (gorge once per sacrifice batch, no implicit per-turn
// limit) queues once for an effect's sacrifice of two Foods, and again for
// a later sacrifice that turn.
func TestSacrificedOnce(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Camellia, the Seedmiser", Count: 1}, {Name: "Carrot Cake", Count: 3}, {Name: "Plains", Count: 8}})
	cm, cc := id["Camellia, the Seedmiser"], id["Carrot Cake"]
	c := sc.perm(cm, 0)
	ab := trigOneormoreSacAb(t, sc, cm)
	sc.s.npend = 0
	sc.s.addSac(sc.slot(sc.perm(cc, 0)))
	sc.s.addSac(sc.slot(sc.perm(cc, 0)))
	sc.s.flushSac(sc.k)
	if n := batchPend(sc, c, ab); n != 1 {
		t.Fatalf("sacrificing two Foods queued %d Camellia triggers, want 1", n)
	}
	sc.s.npend = 0
	sc.s.addSac(sc.slot(sc.perm(cc, 0)))
	sc.s.flushSac(sc.k)
	if n := batchPend(sc, c, ab); n != 1 {
		t.Fatalf("a second sacrifice batch this turn queued %d Camellia triggers, want 1", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("batch state left open: %#x", sc.s.g.batchLatch)
	}
}

// trigOneormoreSacAb is Camellia's SacrificedOnce line: Sacrificed with Tf2Batch and
// no once-each-turn limit (gorge triggerTurnLimitFor).
func trigOneormoreSacAb(t *testing.T, sc *scen, cm CardID) uint8 {
	t.Helper()
	for j, a := range sc.k.t.Cards[cm].Abilities {
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigSacrificed {
			if a.Trig.Flags&kop.TfOncePerTurn != 0 || a.Trig.Flags2&kop.Tf2Batch == 0 {
				t.Fatalf("Camellia's SacrificedOnce flags %#x / %#x, want Tf2Batch only", a.Trig.Flags, a.Trig.Flags2)
			}
			return uint8(j)
		}
	}
	t.Fatal("Camellia has no Sacrificed trigger")
	return 0
}

// TestSacrificedOnceCostBatch: one cost payment sacrificing two Foods is
// one sacrifice batch (gorge emitSacrifices at the cost sites), so Camellia
// queues once; two payments in the same turn are two batches and queue
// twice; a cost batch is closed after the payment.
func TestSacrificedOnceCostBatch(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Camellia, the Seedmiser", Count: 1}, {Name: "Carrot Cake", Count: 4}, {Name: "Plains", Count: 8}})
	cm, cc := id["Camellia, the Seedmiser"], id["Carrot Cake"]
	c := sc.perm(cm, 0)
	ab := trigOneormoreSacAb(t, sc, cm)
	var foods []Obj
	for i := 0; i < 4; i++ {
		foods = append(foods, sc.perm(cc, 0)) // (perm clears the pending queue)
	}
	pay := func(n int) {
		sc.s.bld().e = stackEntry{kind: seSpell}
		sc.s.bld().pickN = 0
		for i := 0; i < n; i++ {
			sc.s.bld().picks[i] = foods[0]
			foods = foods[1:]
			sc.s.bld().pickKs[i] = pkSac
			sc.s.bld().pickN++
		}
		sc.s.paySacsGen(sc.k)
		sc.s.bld().pickN = 0
	}
	sc.s.npend = 0
	pay(2)
	if n := batchPend(sc, c, ab); n != 1 {
		t.Fatalf("one cost sacrificing two Foods queued %d Camellia triggers, want 1", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("cost batch left open: %#x", sc.s.g.batchLatch)
	}
	sc.s.npend = 0
	pay(1)
	pay(1)
	if n := batchPend(sc, c, ab); n != 2 {
		t.Fatalf("two cost payments in one turn queued %d Camellia triggers, want 2", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("cost batch left open: %#x", sc.s.g.batchLatch)
	}
}

// newBLBScenBatch is newBLBScen with card c's first DamageDone trigger
// marked as a batch line (a DamageDoneOnce: no such card is accepted in the
// BLB pool, so Toski's "whenever a creature you control deals combat damage
// to a player" stands in for Ongoing Investigation's "one or more").
func newBLBScenBatch(t *testing.T, deck []cardset.Entry, name string) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "blb")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	cp := *tbl
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cd := &cp.Cards[id[name]]
	cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDamageDone {
			a.Trig.Flags2 |= kop.Tf2Batch
			break
		}
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(&cp, lib[:], lib[:], 1, Config{NoElide: true})
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

// TestDamageDoneOnceCombat: a DamageDone combat-to-a-player line with the
// batch flag (DamageDoneOnce) draws once when three creatures (Toski
// attacks each combat) connect in one combat damage step; without it, once
// per creature.
func TestDamageDoneOnceCombat(t *testing.T) {
	deck := []cardset.Entry{{Name: "Toski, Bearer of Secrets", Count: 1}, {Name: "Seasoned Warrenguard", Count: 2}, {Name: "Plains", Count: 8}}
	for _, batch := range []bool{true, false} {
		var sc *scen
		var id map[string]CardID
		if batch {
			sc, id = newBLBScenBatch(t, deck, "Toski, Bearer of Secrets")
		} else {
			sc, id = newBLBScen(t, deck)
		}
		ts := sc.perm(id["Toski, Bearer of Secrets"], 0)
		a, b := sc.perm(id["Seasoned Warrenguard"], 0), sc.perm(id["Seasoned Warrenguard"], 0)
		h := sc.s.pl[0].handN
		sc.combat(t, []Obj{a, b}, nil)
		want := 3 // Toski attacks each combat too
		if batch {
			want = 1
		}
		got := int(sc.s.pl[0].handN-h) + batchPend(sc, ts, 0)
		for i := 0; i < int(sc.s.sp); i++ {
			if sc.s.stack[i].obj == ts {
				got++ // queued, not yet resolved (two pose the order ask)
			}
		}
		if got != want {
			t.Fatalf("batch=%v: drew %d, want %d (opponent life %d)", batch, got, want, sc.s.pl[1].life)
		}
	}
}

// TestDamagePlus: Valley Flamecaller's "a Lizard ... you control deals
// that much damage plus 1" adds 1 to its own combat damage; with a second
// Flamecaller both match one event, gorge asks for their order (CR 616.1)
// and the kernel stalls (StallReplPlus).
func TestDamagePlus(t *testing.T) {
	deck := []cardset.Entry{{Name: "Valley Flamecaller", Count: 2}, {Name: "Plains", Count: 8}}
	sc, id := newBLBScen(t, deck)
	fc := id["Valley Flamecaller"]
	f := sc.perm(fc, 0)
	life := sc.s.pl[1].life
	sc.combat(t, []Obj{f}, nil)
	if lost := life - sc.s.pl[1].life; lost != 4 {
		t.Fatalf("a 3-power Flamecaller dealt %d, want 4", lost)
	}
	if sc.s.g.stalled != 0 {
		t.Fatalf("stalled %s with one Flamecaller", StallName(sc.s.g.stalled))
	}
	sc, id = newBLBScen(t, deck)
	f = sc.perm(fc, 0)
	sc.perm(fc, 0)
	sc.s.replDamage(sc.k, 0, sc.slot(f), fc, uint8(PlayerTarget0+1), true, 3)
	if sc.s.g.stalled != StallReplPlus {
		t.Fatalf("two Flamecallers on one event: stalled %q, want replplus", StallName(sc.s.g.stalled))
	}
}
