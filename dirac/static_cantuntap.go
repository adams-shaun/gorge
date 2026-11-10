package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugAnyUntapOff is TestAnyUntapBlockedMutation's switch.
var DebugAnyUntapOff bool

// untapBlocked reports whether an any-event "can't become untapped"
// replacement (ReplAnyUntap; Frozen in Ice) matching sl forbids
// the Untap event a caller is about to emit (CR 614.17; gorge cards.ReplUntap
// with no ValidStepTurnToController$ matches every events.Untap). Any match:
// blocked, with no CR 616.1 ask, since the effect is no replacement (CR 502.3,
// 614.1). The untap step does not come through here: untapCands counts its own.
// Stun counters
// are handled by the callers before this point (CR 122.1d: removed instead,
// so before the Untap event exists).
//
//go:noinline
func (s *State) untapBlocked(k *prep, sl Slot) bool {
	if DebugAnyUntapOff || k.gen == nil || !k.gen.anyUntapBlock {
		return false
	}
	if s.perms[sl].flags&pfTapped == 0 {
		return false // no Untap event: gorge TryUntap returns early
	}
	if s.g.nul != 0 && s.untapLocked(s.perms[sl].obj, true) {
		return true // Spider-Woman's lock (act_untaplock.go)
	}
	for pp := uint8(0); pp < MaxSeats; pp++ {
		var bf [MaxPerm]Slot
		m := s.statM.and(s.ctrlM[pp]).with(sl).and(s.ctrlM[pp]) // untapCands' scan shape
		for _, b := range bf[:s.battlefieldOrder(pp, m, bf[:], 0)] {
			if s.genLost(k, b) {
				continue
			}
			cd := &k.t.Cards[s.cardAt(b)]
			for j := range cd.Abilities {
				a := &cd.Abilities[j]
				if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplAnyUntap {
					continue
				}
				if !s.matchPermGen(k, a.Repl.Filter, sl, s.perms[b].ctrl, b, layerFull) {
					continue
				}
				return true
			}
		}
	}
	return false
}
