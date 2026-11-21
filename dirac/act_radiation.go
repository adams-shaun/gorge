package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Rad counters (CR 122.1, 728; gorge effects/counters.go effRadiation and
// effRadiationDrain, rules/turn.go's RadiationDrain trigger). Cold and out of
// line: a table with no OpExt ExtRadiation keeps no array (genPrep.anyRad)
// and nobody ever has a rad counter.
//
// State: coldRad, one count per seat (seat-generic; no seat-count branch).
//
//   - ExtRadiation: each player Obj names gets B rad counters. gorge emits a
//     PlayerCounterChange "RAD", which its AddCounter replacements see as
//     they see poison, energy or contract counters: the kernel applies the
//     plus-N player-counter lines (plusPlayerCtr) and the counter doubler on
//     its controller's own counters (dblCtrYou), as for energy and contracts.
//   - CR 728.1: at the beginning of the precombat main phase of a player who
//     has a rad counter, after the step's printed triggers and the Sagas'
//     lore counters (gorge finishEnteredStep appends it after advanceSagas),
//     a source-less trigger that player controls is queued (an abDelayed
//     entry with obj NoObj and trigAmt radDrainAmt).
//   - It resolves as gorge's effRadiationDrain: the player's top N cards (N
//     the rad counters they have as it resolves) are fixed, then milled one
//     at a time; after each nonland card the player loses 1 life and removes
//     one rad counter. A card no longer in the library is skipped. A mill
//     whose move competes between exile-instead replacements (gorge asks
//     the order mid-drain, the kernel parks such moves) stalls StallRadMill.
//   - Proliferate offers a player with a rad counter and gives them one more
//     (gorge hasRealCounters over the player's counters).

// StallRadMill: the rad drain would mill a card whose graveyard move
// competes between replacements (repl_exilemark.go graveMoveCompetes).
const StallRadMill = 180

func init() { stallNames[StallRadMill] = "radmill" }

// radDrainAmt is the abDelayed entry's trigAmt marking the rad drain trigger
// (speed's is -1, the venture's -2, the monarch's -3 and -4).
const radDrainAmt = -5

// RadiationStats counts the mechanism's events; oracle tests only.
var RadiationStats struct {
	Gained  atomic.Int64 // players given rad counters (one per player per instruction)
	Drains  atomic.Int64 // drain triggers queued
	Milled  atomic.Int64 // cards a drain milled
	Nonland atomic.Int64 // nonland cards a drain milled: 1 life lost and a rad counter removed each
}

var coldRad = newCold[uint16]("rad", func(g *genPrep) int {
	if g.anyRad {
		return MaxSeats
	}
	return 0
})

// radCold is the per-seat rad counters, or nil when the table never gives any.
func (s *State) radCold() []uint16 { return coldRad.of(s) }

// Rad is seat p's rad counter count (oracle).
func (s *State) Rad(p uint8) int {
	if c := s.radCold(); c != nil && p < MaxSeats {
		return int(c[SeatIx(p)])
	}
	return 0
}

// radLine reports whether a gives rad counters (OpExt ExtRadiation).
func radLine(a *kop.Ability) bool {
	for i := range a.Prog {
		if in := &a.Prog[i]; in.Op == kop.OpExt && in.Ext() == kop.ExtRadiation {
			return true
		}
	}
	return false
}

// radiationExt is OpExt ExtRadiation: each player Obj names gets B rad
// counters.
//
//kmanifest:ext ExtRadiation
func (s *State) radiationExt(k *prep, in *kop.Instr) (ok, done bool) {
	q := s.players(k, in.Obj)
	n := s.value(k, in.B)
	for i := uint8(0); i < q.n; i++ {
		s.addRad(k, q.s[i], n)
	}
	return true, true
}

// addRad gives seat p n rad counters, after the player-counter replacements.
// A negative n (Feral Ghoul dying with negative power under Nuclear
// Fallout's -X/-X) gives nothing: CR 107.1b treats the negative amount as
// zero, and gorge's effRadiation clamps it before emitting.
func (s *State) addRad(k *prep, p uint8, n int) {
	c := s.radCold()
	if c == nil || n <= 0 || p >= MaxSeats {
		return
	}
	n = s.plusPlayerCtr(k, p, n) // repl_plrcounter.go
	if k.gen != nil && k.gen.anyRepl&rbDblCtrYou != 0 {
		for d := s.dblCtrYou(k, s.counterAdder()); d > 0 && n <= 0xffff; d-- {
			n *= 2
		}
	}
	c[SeatIx(p)] = uint16(min(int(c[SeatIx(p)])+n, 0xffff))
	RadiationStats.Gained.Add(1)
}

// radMain1 queues the active player's drain trigger at the beginning of
// their precombat main phase (CR 728.1); beginStep calls it at both main
// phases, and a table that gives no rad counters returns at once.
func (s *State) radMain1(k *prep) {
	if k.gen == nil || !k.gen.anyRad || s.step != kop.StepMain1 {
		return
	}
	s.radMain1Queue()
}

//go:noinline
func (s *State) radMain1Queue() {
	c := s.radCold()
	if c == nil || c[SeatIx(s.active)] == 0 || s.pl[SeatIx(s.active)].lost {
		return
	}
	s.enqueue(NoObj, 0, 0, abDelayed, s.active, NoObj, s.active, radDrainAmt)
	RadiationStats.Drains.Add(1)
}

// radTrig reports whether e is the rad drain trigger.
func radTrig(e *stackEntry) bool { return e.obj == NoObj && e.trigAmt == radDrainAmt }

// resolveRadTrig resolves the drain (gorge effRadiationDrain): mill as many
// cards as the controller has rad counters; each nonland card milled costs 1
// life and a rad counter.
//
//go:noinline
func (s *State) resolveRadTrig(k *prep, e *stackEntry) {
	p := e.ctrl
	c := s.radCold()
	if c == nil || p >= MaxSeats {
		return
	}
	pl := &s.pl[SeatIx(p)]
	var top [MaxObj]Obj
	m := copy(top[:min(int(c[SeatIx(p)]), len(top))], pl.lib[pl.libLo:pl.libHi])
	for _, o := range top[:m] {
		ob := &s.objs[o]
		if ob.zone != kop.ZoneLibrary || ob.owner() != p {
			continue // gone from the library since the top was fixed
		}
		if s.graveMoveCompetes(k, o) {
			s.capFull(StallRadMill)
			return
		}
		land := k.cards[ob.card].types&kop.TLand != 0
		pl.libRemove(o)
		s.touch(o)
		ob.zone = kop.ZoneNone
		s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false, false)
		RadiationStats.Milled.Add(1)
		if land {
			continue
		}
		s.loseLife(k, p, 1)
		if c[SeatIx(p)] > 0 {
			c[SeatIx(p)]--
		}
		RadiationStats.Nonland.Add(1)
	}
}
