package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The land-play lock (Memory Vessel's NoLand: Mode$ CantPlayLand |
// Player$ Player | Origin$ Hand on the dig Effect's emblem): gorge
// playLandForbidden (rules/play_land_restriction.go) is the offer and
// commit-time rule of every land play; the kernel reads the same statics at
// the hand walk's offer (legal.go). The lowering admits only Origin$ Hand
// (cast_freeland.go's queued plays and the permitted exile plays run no
// such gate, so a lock naming another zone needs its own read) and no
// ValidCard$ filter (gorge's empty-spec arm is the only lowered shape), so
// the static's Zone is always ZoneHand and its Filter 0.

// cantPlayLandLock reports whether a StatCantPlayLand static forbids p
// playing card c (object o) out of its current zone: an active static (a
// battlefield permanent's or an emblem's) whose player scope (StRaiseYou
// its controller, StRaiseOpp its opponents, neither anyone), condition and
// origin zone all hold. gorge playLandForbidden: the Player$/ValidPlayer$
// spec matched from the static's controller, the Origin$ zone list read
// against the play's zone (the object's current zone at every offer).
//
//go:noinline
func (s *State) cantPlayLandLock(k *prep, p uint8, c CardID, o Obj) bool {
	if k.gen == nil || !k.gen.anyCantPlayLand {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantPlayLand {
			return true
		}
		stt := &a.Static
		if stt.StFlags&kop.StRaiseYou != 0 && st.ctrl != p || stt.StFlags&kop.StRaiseOpp != 0 && st.ctrl == p {
			return true
		}
		if stt.Cond2 != 0 && !s.condEval(k, stt.Cond2, s.ctxStatic(st)) {
			return true
		}
		if o != NoObj && s.objs[o].zone != stt.Zone {
			return true // Origin$: the play's zone is not the restricted one
		}
		hit = true
		return false
	})
	return hit
}
