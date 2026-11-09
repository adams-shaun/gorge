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
	if a.Repeats() {
		order, n := a.AskOrder(mode)
		for i := 0; i < n; i++ {
			mask |= 1 << order[i]
		}
		return mask, n
	}
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

// StackTargetOrder is the order gorge lists a stack entry's target slots in:
// each chosen mode's root slot, then the chain links (Object.Targets, then
// Object.SubTargets). Slots are laid out mode by mode, so this moves a link only
// when a later chosen mode also targets; Clash of the Eikons is the one such
// card. The order has no rules meaning (CR 601.2c): resolution reads each slot
// by number, and this is only how the oracle lines the two lists up.
func StackTargetOrder(a *kop.Ability, mode uint8) (order [kop.MaxTargets]uint8) {
	roots, _ := charmRootSlots(a, mode)
	n := 0
	if a.Repeats() {
		// the instances' groups in printed order, then the unused slots
		o, no := a.AskOrder(mode)
		copy(order[:], o[:no])
		n = no
		for j := range order {
			if roots&(1<<j) == 0 {
				order[n] = uint8(j)
				n++
			}
		}
		return order
	}
	for pass := 0; pass < 2; pass++ {
		for j := range order {
			if (roots&(1<<j) != 0) == (pass == 0) {
				order[n] = uint8(j)
				n++
			}
		}
	}
	return order
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
