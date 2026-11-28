package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Alchemist's Gambit's ExtraTurnDelayedTrigger$ (kgen lower_addturnupkeep.go,
// OpAddTurn B=1): "During that turn, damage can't be prevented." gorge
// registers a Static$ True Mode$ Phase | Phase$ Upkeep delayed trigger at the
// -1 ExtraTurn consumption with the granted turn as MinTurn (events.Apply
// foldExtraTurn); it fires at the first upkeep that actually happens on or
// after that turn (a skipped upkeep's StepChange never lands, Eon Hub;
// rules checkDelayedTriggers). There its DB$ Effect registers a global
// CantPreventDamage restriction (no Affected$: every damage, combat or not;
// rules cantPreventDamage) that ends with the turn and, on the uncleaved
// face, arms EndLose at that turn's end step. The kernel marks the grant (one
// bit per exQ entry, beside exRide), turns it into a pending record when
// exTake begins its turn, and arms it at the first upkeep entry on or after
// that turn (upkeepStep): noPrev reads the armed turn
// (static_cantprevent.go), and the loss rides exRideArm from there.

// maxExNoPrev caps the pending upkeep riders: one per granted turn whose
// upkeep has not happened yet.
const maxExNoPrev = 4

// exNoPrevPend is one consumed grant's rider waiting for its upkeep: seat p
// (the rider's controller), src the spell the loss names its source, lose
// the uncleaved face's end-step loss, from the first turn it may fire.
type exNoPrevPend struct {
	src  Obj
	from uint16
	p    uint8
	lose bool
}

// exNoPrevRec is the rider's state: grants bit i, exQ[i]'s grant carries the
// rider (lose bit i, with the end-step loss); pend, the consumed grants
// waiting for an upkeep; turn, the turn number damage can't be prevented in
// (0 none).
type exNoPrevRec struct {
	grants uint32
	lose   uint32
	turn   uint16
	n      uint8
	pend   [maxExNoPrev]exNoPrevPend
}

// coldExNoPrev holds the one record, only for a table with the rider (a cold
// region: a State field would fail TestNoNewFixedStateFields).
var coldExNoPrev = newCold[exNoPrevRec]("exnoprev", func(g *genPrep) int {
	if g.uses.has(useExNoPrev) {
		return 1
	}
	return 0
})

// useExNoPrev: some ability grants an extra turn with the rider.
var useExNoPrev = newGenUse(tableAnyExNoPrev)

func tableAnyExNoPrev(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpAddTurn && in.B == kop.Const(1) {
					return true
				}
			}
		}
	}
	return false
}

// exPushOp is OpAddTurn's grant: exPush with C's end-step rider, and B=1
// marking the pushed grant as carrying the upkeep rider -- whose C loss waits
// for that upkeep instead of arming with the turn as Final Fortune's does.
func (s *State) exPushOp(p uint8, in *kop.Instr, src Obj) {
	n := s.g.exN
	up := in.B == kop.Const(1)
	s.exPush(p, in.C != 0 && !up, src)
	if c := coldExNoPrev.of(s); c != nil && up && s.g.exN > n {
		c[0].grants |= 1 << n
		if in.C != 0 {
			c[0].lose |= 1 << n
		}
	}
}

// exNoPrevTake reads and removes grant ix's marks as exTake spends it; shift
// (the FIFO mutation's order) drops entry 0 and moves the rest down.
func (s *State) exNoPrevTake(ix uint8, shift bool) (on, lose bool) {
	c := coldExNoPrev.of(s)
	if c == nil {
		return false, false
	}
	on, lose = c[0].grants&(1<<ix) != 0, c[0].lose&(1<<ix) != 0
	if shift {
		c[0].grants >>= 1
		c[0].lose >>= 1
	} else {
		c[0].grants &^= 1 << ix
		c[0].lose &^= 1 << ix
	}
	return on, lose
}

// exNoPrevPendAdd: the turn exTake is about to begin is p's granted turn
// with the upkeep rider (beginTurn advances s.turn by one); it waits for the
// first upkeep from that turn on, gorge's MinTurn.
func (s *State) exNoPrevPendAdd(p uint8, lose bool, src Obj) {
	c := coldExNoPrev.of(s)
	if c == nil {
		return
	}
	if c[0].n >= maxExNoPrev {
		s.capFull(StallExTurn)
		return
	}
	c[0].pend[c[0].n] = exNoPrevPend{src: src, from: s.turn + 1, p: p, lose: lose}
	c[0].n++
}

// exNoPrevUpkeep is the upkeep entry's rider (upkeepStep): every pending
// record whose turn has come fires -- damage can't be prevented this turn,
// and the uncleaved face's loss arms for this turn's end step. A record whose
// seat has lost is dropped, as gorge's drain drops a lost controller's
// registration.
func (s *State) exNoPrevUpkeep() {
	c := coldExNoPrev.of(s)
	if c == nil || c[0].n == 0 {
		return
	}
	keep := uint8(0)
	for i := uint8(0); i < c[0].n; i++ {
		r := c[0].pend[i]
		if s.turn < r.from {
			c[0].pend[keep] = r
			keep++
			continue
		}
		if s.pl[r.p].lost {
			continue
		}
		c[0].turn = s.turn
		if r.lose {
			s.exRideArm(r.p, r.src)
		}
	}
	for i := keep; i < c[0].n; i++ {
		c[0].pend[i] = exNoPrevPend{}
	}
	c[0].n = keep
}

// exNoPrevNow reports whether this turn is one whose upkeep fired the rider:
// damage can't be prevented.
func (s *State) exNoPrevNow() bool {
	c := coldExNoPrev.of(s)
	return c != nil && c[0].turn != 0 && c[0].turn == s.turn
}
