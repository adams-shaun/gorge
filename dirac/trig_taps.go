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

// manaTapped: sl just became tapped as the cost of a mana ability (CR 605.1b: the
// ability does not use the stack, the Taps trigger it causes waits for priority, CR
// 603.3). Only a table with a Taps listener beside a granted mana ability can tap one
// for mana; the rest never reach the trigger machinery.
func (s *State) manaTapped(k *prep, sl Slot) {
	if k.gen != nil && k.gen.anyManaOnce {
		// the first mana ability's own once-only mark is spent as it produces
		// (Loot, the Pathfinder's Exhaust mana ability; CR 702.177a)
		if pc := s.manaPCGen(k, sl); pc.manaAb >= 0 {
			if cd := &k.t.Cards[s.cardAt(sl)]; int(pc.manaAb) < len(cd.Abilities) {
				if a := &cd.Abilities[pc.manaAb]; a.Cost.Flags&kop.CostOncePerGame != 0 {
					s.onceUse(k, sl, a)
				}
			}
		}
	}
	if k.gen != nil && k.gen.manaTaps {
		s.fireTaps(k, sl)
	}
	if k.gen != nil && k.gen.anyManaRider {
		s.manaRider(k, sl) // a counter rider resolves with the mana (mana_rider.go)
	}
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
