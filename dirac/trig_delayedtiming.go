package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dprog.n timing bits (OpDelayedProgram A: kop.DelayUpkeep,
// kop.DelayYourUpkeep), above the object count.
const (
	dpWhenShift = 6
	dpWhenMask  = 3 << dpWhenShift
)

// dpWhen is an OpDelayedProgram's timing as dprog.n bits.
func dpWhen(in *kop.Instr) uint8 {
	if in.A.N == kop.DelaySpellCast {
		return dpSpell // trig_delayedspell.go: never due at a step
	}
	if in.A.N == kop.DelayDiesThisTurn {
		return dpDies // trig_delayeddies.go: never due at a step
	}
	return uint8(in.A.N&3) << dpWhenShift
}

// tableDelaysUpkeep reports whether some program of t registers an upkeep
// delayed program (genPrep.anyDelayUp).
func tableDelaysUpkeep(t *Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpDelayedProgram && in.A.N != 0 {
					return true
				}
			}
		}
	}
	return false
}

// fireDelayedUpkeep fires the upkeep delayed programs as an upkeep begins,
// after the step's own Phase triggers (gorge checkDelayedTriggers runs on
// the StepChange after the face scan): kop.DelayUpkeep ones always,
// kop.DelayYourUpkeep ones when their controller is the active player
// (the registration's ValidPlayer$ You against the active player). The
// others stay registered.
//
//go:noinline
func (s *State) fireDelayedUpkeep(k *prep) {
	s.fireDelayedWhen(k, 1)
}

// fireDelayedWhen enqueues, in registration order, the delayed programs due
// now -- at=0: the end step's (no timing bits); at=1: the upkeep's; at=2:
// the end of combat step's (kop.DelayEndCombat) -- and
// keeps the rest in order with their LKI indexes. Each LKI index is copied
// into its pending trigger before the pool shrinks (zone_lki.go).
//
//go:noinline
func (s *State) fireDelayedWhen(k *prep, at int) {
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		d := s.g.dp[i]
		w := d.n & dpWhenMask >> dpWhenShift
		due := d.n&(dpSpell|dpDies) == 0 && w == 0 && at == 0 ||
			at == 1 && (w == kop.DelayUpkeep || w == kop.DelayYourUpkeep && s.active == d.ctrl) ||
			at == 2 && w == kop.DelayEndCombat
		if !due {
			s.g.dp[keep], s.g.dpLKI[keep] = d, s.g.dpLKI[i]
			keep++
			continue
		}
		m := s.npend
		ro, remVer, remSet := s.delayRem(k, &d)
		var amt int16
		if d.n&dpCountMask == 2 {
			amt = s.dpSecond(&d) // Mobilize 2's second token (kw_mobilize.go)
		}
		sv := s.objs[d.src].ver
		if d.n&dpTrack != 0 {
			sv, amt = d.vers[0], dpTrackAmt // a tracked promise: resolves doing nothing if the object left
		}
		s.enqueue(d.src, sv, d.card, d.ab, d.ctrl, ro, d.ctrl, amt)
		if s.npend > m {
			s.pend[m].li = s.g.dpLKI[i]
			s.pend[m].remVer, s.pend[m].skw = remVer, s.pend[m].skw&^skwRem|boolBit(remSet, skwRem)
			if d.n&dpMany != 0 {
				s.dpManyCarry(&d, &s.pend[m]) // trig_delayedmany.go
			}
		}
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
}

// Kernel arms: cold and out
// of line, reached from execCold3's default branch or behind one prep bit.

// fireDelayedEndCombat fires the end-of-combat delayed programs
// (kop.DelayEndCombat: gorge Phase$ EndCombat, ValidPlayer$ Player) as an
// end of combat step begins, after the step's own Phase triggers (gorge
// checkDelayedTriggers on the StepChange). The others stay registered.
//
//go:noinline
func (s *State) fireDelayedEndCombat(k *prep) {
	s.fireDelayedWhen(k, 2)
}
