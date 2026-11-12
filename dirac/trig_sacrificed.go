package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fireSac fires Sacrificed triggers for the permanent at sl, called just
// before a sacrifice moves it (cost, effect or mana; gorge sacrificedMatches
// on the sacrifice's zone-change event, matched against the permanent's
// last known information). One predictable branch when no card listens.
func (s *State) fireSac(k *prep, sl Slot) {
	s.noteSac(k, sl)
	if k.gen != nil && k.gen.anyRemSac {
		s.sacRemember(k, sl)
	}
	if s.subs[kop.TrigSacrificed].any() {
		s.fireSacCold(k, sl)
	}
	if k.gen != nil && k.gen.graveSac {
		s.fireGraveSac(k, sl)
	}
}

//go:noinline
func (s *State) fireSacCold(k *prep, sl Slot) {
	p := &s.perms[sl]
	s.fireObj(k, kop.TrigSacrificed, sl, p.obj, p.ctrl, 0)
}

// fireSacAll is fireSac over a simultaneous sacrifice (OpSacrificeAll), in
// leaveAll's order: seat 0's battlefield in entry order, then seat 1's. It
// also tallies every sacrifice (noteSac), listener or not.
func (s *State) fireSacAll(k *prep, hit slotMask) {
	var ord [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		n = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], n)
	}
	for _, sl := range ord[:n] {
		s.noteSac(k, sl)
	}
	if k.gen != nil && k.gen.graveSac {
		defer func() {
			for _, sl := range ord[:n] {
				s.fireGraveSac(k, sl)
			}
		}()
	}
	if !s.subs[kop.TrigSacrificed].any() {
		return
	}
	for _, sl := range ord[:n] {
		s.fireSacCold(k, sl)
	}
}
