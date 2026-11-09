package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A library-top play permission whose alternative cost is life (Gwenom,
// Remorseless: "If you cast a spell this way, pay life equal to its mana
// value rather than pay its mana cost"; kop.StMayPayLife, CR 118.9). It
// reuses the pay-life record's pricing (cast_mayplaypaylife.go) on a static:
// priorityMayPlay offers the cast free of mana when the life can be paid
// (payLifeOfferable) and castFlagsGen marks the build (bld().payLifeMV), so
// the cast pays the mana value in life with its other costs. The permission
// is the static's, so no record is spent. Cold: every site sits behind the
// static's flag.

// FeatMayPayLifeCast reports whether player p's play of o from outside the
// hand goes through a permission whose alternative cost is life (a pay-life
// record, or a library-top static with kop.StMayPayLife). gorge offers that
// cast as the permission's alternative (AltCostIndex 1), where an untyped
// may-play cast is the plain one: the canonical form follows.
func (s *State) FeatMayPayLifeCast(p uint8, o Obj) bool {
	k := s.k()
	if k.gen == nil || s.objs[o].zone == kop.ZoneHand {
		return false
	}
	ob := &s.objs[o]
	for i := 0; i < int(s.g.nmay); i++ {
		if r := s.g.may[i]; r.obj == o && r.ctrl == p && r.ver == ob.ver && !ownWalkerRecord(r) && r.payLife() {
			return true
		}
	}
	found := false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		found = st.a.Static.StFlags&kop.StMayPayLife != 0 && s.payLifeOfferable(k, p, o)
		return !found
	})
	return found
}

// StallPayLifeShared: a pay-life library static and a second play permission
// admit the same card (Gwenom's emblem beside Madame Web, Clairvoyant). gorge
// offers two casts there, the plain one paying mana and the alternative paying
// life (AltCostIndex 1); the kernel joins the permissions into one offer, so
// the game stalls loudly rather than let the cast skip both payments.
const StallPayLifeShared = 153

func init() { stallNames[StallPayLifeShared] = "paylifeshared" }
