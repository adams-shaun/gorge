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
	found := false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		found = st.a.Static.StFlags&kop.StMayPayLife != 0 && s.payLifeOfferable(k, p, o)
		return !found
	})
	return found
}

// StallPayLifeShared: a pay-life library static and a different play
// permission admit the same card (Gwenom's emblem beside Madame Web,
// Clairvoyant). gorge offers two casts there, the plain one paying mana and
// the alternative paying life (AltCostIndex 1); the kernel joins the
// permissions into one offer, so the game stalls loudly rather than let the
// cast skip both payments. Pay-life statics alone (Gwenom's emblem twice,
// Peter Parker's Camera copying its trigger) stall nowhere: gorge lists one
// life alternative per Effect and fanPayLifeAlts offers one cast each.
const StallPayLifeShared = 153

func init() { stallNames[StallPayLifeShared] = "paylifeshared" }

// payLifeAltMax bounds the pay-life alternatives offered apart: the canonical
// alternative-cost index is a nibble (canon/translate.go).
const payLifeAltMax = 15

// fanPayLifeAlts turns the one cast offerPlay added at d.Options[n0] into
// alts casts, targets 1..alts: the 1-based AltCostIndex of gorge's option for
// each pay-life Effect (CR 118.9). All cost the same life, so the one
// affordability check stands for each; a land's play-land option stays single
// (an alternative cost applies to a spell). The apply side ignores the target
// of a may-play cast.
func fanPayLifeAlts(d *Decision, n0, alts int) {
	if int(d.N) != n0+1 || d.Options[n0].Kind() != AkCast {
		return
	}
	a := d.Options[n0]
	d.Options[n0] = MakeAction(AkCast, a.Source(), 1, a.Mode())
	for i := 2; i <= alts; i++ {
		d.add(MakeAction(AkCast, a.Source(), uint8(i), a.Mode()))
	}
}
