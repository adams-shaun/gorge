package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Banding's grant read (CR 702.22). KwMask is full (kop/keywords.go), so a
// granted banding is kop.Override.Banding on the active override records,
// not a keyword bit: kwGen's mask cannot carry it and the projection maps
// the keyword beside the record, like CantAttack. combat_band.go's CR 702.22c
// band ask and band formation (CR 702.22d-k) call this read. The
// record's lifetime is the grant's: a Pump Duration$
// UntilEndOfCombat record carries UntilEndCombat and static_ovendcombat.go
// drops it leaving the end-of-combat step, a Pump's until-end-of-turn record
// ends with the turn, and a battlefield Continuous static's AddKeyword$
// Banding (Dire Wolves, Banding Sliver) is the Banding of that static's
// Override record, read while the static applies (bandingStatic).

// hasBanding reports whether an active override record grants sl banding. A
// grant older than the permanent's "loses all abilities" effect is removed
// with the rest of layer 6 (kwGen's override loop).
func (s *State) hasBanding(k *prep, sl Slot) bool {
	lts, lost := s.lossAt(k, sl)
	found := false
	s.eachOverrideTs(k, sl, func(o *kop.Override, ts uint16) {
		if (!lost || ts >= lts) && o.Banding {
			found = true
		}
	})
	return found || s.bandingStatic(k, sl, lost, lts)
}

// bandingStatic reports whether a battlefield Continuous static whose
// Override record carries Banding applies to sl: its condition holds, its
// source still has its abilities, its filter matches sl, and sl did not lose
// all abilities after the source's timestamp (CR 613.7, as hasBanding's
// records).
func (s *State) bandingStatic(k *prep, sl Slot, lost bool, lts uint16) bool {
	for m := s.statM; m.any(); m = m.rest() {
		src := m.first()
		if s.genLost(k, src) {
			continue
		}
		sp := &s.perms[src]
		cd := &k.t.Cards[s.cardAt(src)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Override == 0 ||
				!k.t.Overrides[a.Static.Override-1].Banding {
				continue
			}
			if lost && s.layerTs(src) < lts {
				continue
			}
			st := statSrc{a: a, sl: src, ctrl: sp.ctrl, obj: sp.obj}
			if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, src, layerKw) {
				return true
			}
		}
	}
	return false
}
