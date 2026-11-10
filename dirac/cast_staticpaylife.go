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
		if r := *s.mayAt(i); r.obj == o && r.ctrl == p && r.ver == ob.ver && !ownWalkerRecord(r) && r.payLife() {
			return true
		}
	}
	return s.payLifeStaticAdmits(k, p, o)
}

// payLifeStaticAdmits reports whether a library-top static with
// kop.StMayPayLife lets p play o and the life covers its mana value.
func (s *State) payLifeStaticAdmits(k *prep, p uint8, o Obj) bool {
	found := false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		found = st.a.Static.StFlags&kop.StMayPayLife != 0 && s.payLifeOfferable(k, p, o)
		return !found
	})
	return found
}

// castLifeOnly reports whether a may-play cast with mode is the life
// alternative of a pay-life static admitting o (castMayPlay|castFree beside
// it): castFlagsGen then walks to that static only, so the build pays the
// life (bld().payLifeMV) whatever permission is walked first.
func (s *State) castLifeOnly(k *prep, p uint8, o Obj, mode uint8) bool {
	return mode&castFree != 0 && s.objs[o].zone != kop.ZoneHand && s.payLifeStaticAdmits(k, p, o)
}

// castSkipsStatic reports whether castFlagsGen passes over static st for a
// cast with mode: a plain cast (no castFree) never goes through a pay-life
// static, so it neither pays life nor spends that permission (gorge's
// AltCostIndex 0 beside Gwenom's alternative, priorityMayPlay); the life
// alternative (lifeOnly) goes through the pay-life static only.
func castSkipsStatic(st statSrc, mode uint8, lifeOnly bool) bool {
	life := st.a.Static.StFlags&kop.StMayPayLife != 0
	if mode&castFree == 0 {
		return life
	}
	return lifeOnly && !life
}

// StallPayLifeShared: a pay-life library static and a second permission that
// waives the mana cost admit the same card. gorge offers the plain cast and the
// life alternative (AltCostIndex 1) apart; the kernel tells the two apart by
// castFree, which both would carry here, so the game stalls loudly rather than
// let the cast skip the life. Beside a permission that does not waive the mana
// (Madame Web, Clairvoyant) the two casts are offered apart (priorityMayPlay).
const StallPayLifeShared = 153

func init() { stallNames[StallPayLifeShared] = "paylifeshared" }
