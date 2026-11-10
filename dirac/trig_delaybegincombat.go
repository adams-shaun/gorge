package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Delayed "at the beginning of the next combat this turn" triggers
// (CR 603.7, 603.7c): gorge's DelayedTrigger Mode$ Phase, Phase$ BeginCombat,
// ThisTurn$ True, RememberObjects$ Targeted (The Wrath of Khan's chapter I:
// gain control of the damaged creature until end of turn). The registration is
// an ordinary dprog record over the remembered object, flagged dpExtBeginCombat
// in dprog.ext (n's timing bits are full): it is due as the beginning of combat
// step begins, after that step's own Phase triggers (gorge checkDelayedTriggers
// on the StepChange), fires once, and is dropped at the next turn change, so a
// registration made once this turn's beginning of combat has passed never
// fires. An object that changed zones since registration (the creature died to
// the damage) is a new one with nothing to act on (delayRem; CR 400.7).

// dpExtBeginCombat marks a dprog record the beginning-of-combat timing
// (kop.DelayBeginCombat).
const dpExtBeginCombat uint8 = 1

// DebugDelayBeginCombatOff is TestTheWrathOfKhanMutation's switch: a due
// beginning-of-combat program is dropped without running. Never set in
// production.
var DebugDelayBeginCombatOff bool

// dpExt is an OpDelayedProgram's dprog.ext.
func dpExt(in *kop.Instr) uint8 {
	if in.A.N == kop.DelayBeginCombat {
		return dpExtBeginCombat
	}
	if in.A.N == kop.DelayYourEndStep {
		return dpExtYourEndStep
	}
	if in.A.N == kop.DelaySpellUncounter {
		return dpExtUncounter // repl_uncounterable.go
	}
	return 0
}

// fireDelayedBeginCombat fires the beginning-of-combat delayed programs as
// the step begins. The others stay registered.
//
//go:noinline
func (s *State) fireDelayedBeginCombat(k *prep) {
	s.fireDelayedWhen(k, 3)
}

// dropDelayedBeginCombat drops every beginning-of-combat registration at the
// turn change (ThisTurn$ True), keeping the other records in order. A
// controller's-next-end-step registration survives (its own end step may be
// in a later turn).
//
//go:noinline
func (s *State) dropDelayedBeginCombat() {
	n := int(s.g.ndp)
	keep := 0
	for i := 0; i < n; i++ {
		if s.g.dp[i].ext != dpExtBeginCombat {
			s.g.dp[keep], s.g.dpLKI[keep] = s.g.dp[i], s.g.dpLKI[i]
			keep++
		}
	}
	for i := keep; i < n; i++ {
		s.g.dp[i], s.g.dpLKI[i] = dprog{}, 0
	}
	s.g.ndp = uint8(keep)
}
