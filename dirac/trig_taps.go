package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fireTaps fires TrigTaps ("whenever this becomes tapped", gorge Mode$
// Taps) for the permanent at sl that just became tapped (not as it
// entered tapped); its controller is the trigger player and the tapper
// (gorge's attack, cost, convoke, sneak and regeneration taps).
//
//go:noinline
func (s *State) fireTaps(k *prep, sl Slot) {
	s.fireTapsBy(k, sl, s.perms[sl].ctrl)
}

// fireTapsBy is fireTaps with the tapper (gorge TapActor, read by a Taps
// ValidPlayer$): it rides fireObj's amt argument for TrigTaps only, and the
// trigger is queued with amt 0 (objTrigGen).
//
//go:noinline
func (s *State) fireTapsBy(k *prep, sl Slot, tapper uint8) {
	p := &s.perms[sl]
	s.fireObj(k, kop.TrigTaps, sl, p.obj, p.ctrl, int16(tapper))
}

// fireTapsMask is fireTaps over the permanents of m, in battlefield order
// (the attacking player's list: gorge emits a Tap per attacker of the
// declaration).
//
//go:noinline
func (s *State) fireTapsMask(k *prep, m slotMask) {
	var ord [MaxPerm]Slot
	n := s.listenerOrder(m, &ord)
	for _, sl := range ord[:n] {
		s.fireTaps(k, sl)
	}
}
