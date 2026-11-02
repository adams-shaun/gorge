package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A multi-pick charm mode with two target declarations:
// "target creature you control fights target creature an opponent controls".
// gorge's combined ask (target_modes.go askCharmModeTargets) is over the
// chosen modes' ROOT declarations only, one group per mode; a mode's chain
// links are announced after it, one KTarget each (castSubAskLinks,
// target_unique_chain.go), in chosen-mode then chain order. kgen
// (charmMultiTargets) guarantees the lowest slot a mode owns is its root's.

// DebugCharmTwoTgtAllSlots makes the combined ask own every chosen slot, the
// chain links included (the behaviour before this file); the mutation test
// asserts the lockstep catches it.
var DebugCharmTwoTgtAllSlots bool

// charmRootSlots is the combined ask's slots: for each chosen mode (a set bit
// of mode) the lowest slot its TgtModes bit owns.
func charmRootSlots(a *kop.Ability, mode uint8) (mask uint8, n int) {
	if DebugCharmTwoTgtAllSlots {
		return charmSlots(a, mode)
	}
	for m := uint8(0); m < 8; m++ {
		if mode&(1<<m) == 0 {
			continue
		}
		for j := 0; j < int(a.NTargets); j++ {
			if a.TgtModes[j]&(1<<m) != 0 {
				if mask&(1<<j) == 0 {
					mask |= 1 << j
					n++
				}
				break
			}
		}
	}
	return
}

// charmChainNext is the first slot at or after from that a chosen mode owns
// and the combined ask did not (a chain link); kop.MaxTargets when none.
func charmChainNext(a *kop.Ability, mode uint8, from int) int {
	roots, _ := charmRootSlots(a, mode)
	all, _ := charmSlots(a, mode)
	for j := from; j < int(a.NTargets); j++ {
		if all&^roots&(1<<j) != 0 {
			return j
		}
	}
	return kop.MaxTargets
}

// charmRootAsked reports whether slot ti was already announced by this
// build's combined ask (a pure function of the ability, the chosen modes and
// the pick count, as charmCombined computes it).
//
//go:noinline
func (s *State) charmRootAsked(k *prep, a *kop.Ability, ti int) bool {
	if DebugCharmSlotOrder || max8(a.ModesPick, s.twModes(k)) < 2 {
		return false
	}
	mask, n := charmRootSlots(a, s.bld().e.mode)
	return n >= 2 && mask&(1<<ti) != 0 && !charmSeqSlots(a, s.bld().e.mode)
}
