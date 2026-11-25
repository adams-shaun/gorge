package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugExchangeControlOff is the mutation switch: the exchange does nothing.
var DebugExchangeControlOff bool

// exchangeControl is ExtExchangeControl (CR 701.12, gorge effExchangeControl):
// the two permanents named by the target slots in in.C swap controllers for
// good. The pair is all-or-nothing: a target that was illegal as the
// resolution began, has left the battlefield or repeats another leaves fewer
// than two, and two permanents under one controller change nothing. Each
// change is its own permanent-duration control record (CR 613.1b), the first
// pick's first, as gorge registers them.
//
//go:noinline
func (s *State) exchangeControl(k *prep, in *kop.Instr) {
	if DebugExchangeControlOff {
		return
	}
	var pair [MaxPerm]Slot
	if in.Obj == kop.RefSelf {
		// Defined$ Self: the source, then the one target (gorge resolves
		// Defined$ first); a gone source or a self-target is no pair
		self := s.objRef(k, kop.RefSelf)
		if self == NoSlot || s.eachDamageSlots(k, uint8(in.C), 0, &pair) != 1 || pair[0] == self {
			return
		}
		pair[1], pair[0] = pair[0], self
	} else if s.eachDamageSlots(k, uint8(in.C), 0, &pair) != 2 {
		return
	}
	a, b := pair[0], pair[1]
	ca, cb := s.perms[a].ctrl, s.perms[b].ctrl
	if ca == cb {
		return
	}
	if int(s.g.nfx)+2 > maxFx {
		s.stall(StallTables)
		return
	}
	you := s.res.e.ctrl
	s.touchSl(a)
	s.gainControl(a, cb, you, kop.DurPermanent, NoObj, 0, false)
	s.touchSl(b)
	s.gainControl(b, ca, you, kop.DurPermanent, NoObj, 0, false)
}
