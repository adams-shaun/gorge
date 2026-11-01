package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// untapRemembered is OpUntap with Obj$ Remembered: gorge's effUntap loops
// the whole Defined$ set (engine/effects/untap.go, the UntapType$ == ""
// arm; Defined$ Remembered is the resolution's whole remembered set), but
// the kernel's objRef resolves one object (rem[0]). Hunting Wilds' kicked
// "untap all Forests put onto the battlefield this way" is the carrier.
// untapSlot keeps the stun-counter substitution and the net-lock drop.
//
//go:noinline
func (s *State) untapRemembered(k *prep, in *kop.Instr) {
	var objs [maxRem]Obj
	n := copy(objs[:], s.g.rem[:s.g.nrem])
	for i := 0; i < n; i++ {
		o := objs[i]
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		s.untapSlot(k, s.objs[o].slot)
	}
}
