package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Granted prowess (CR 702.108b, 113.2c): a Continuous static's AddKeyword$
// Prowess -- Bria's "other creatures you control have prowess", Sokka's
// "other Allies you control have menace and prowess". Each instance of
// prowess triggers separately, so a creature that prints prowess fires its
// own face trigger and then one more per granting static, and a recipient
// under two such statics fires twice. gorge appends one granted trigger per
// matching ContinuousEffect after the face walk, with no dedupe against the
// printed keyword (rules checkGrantedStaticTriggersUsing).

// prowessGrantCount is how many kernel-native prowess triggers the permanent
// at sl fires on a noncreature cast: one per Continuous static that grants
// prowess to it. The printed keyword is never counted, and no grant is
// deduped against another (a KwMask holds no multiplicity, so this walks the
// statics rather than reading kw).
//
//go:noinline
func (s *State) prowessGrantCount(k *prep, sl Slot) int {
	if k.gen == nil || !k.gen.anyProwessGrant {
		return 0
	}
	lts, lost := s.lossAt(k, sl)
	n := 0
	s.eachStaticC(k, sclsKw, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous || a.Static.Kw&kop.KwProwess == 0 {
			return true
		}
		if lost && s.grantTs(st) < lts {
			return true
		}
		if s.statCond(k, st) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerKw) {
			n++
		}
		return true
	})
	if n == 0 && s.kw(k, sl)&kop.KwProwess != 0 && k.cards[s.cardAt(sl)].kw&kop.KwProwess == 0 {
		n = 1 // a non-static grant (until end of turn): one instance
	}
	if DebugProwessOnce && n > 1 {
		n = 1
	}
	return n
}

// DebugProwessOnce caps the granted prowess triggers at one per permanent
// (the pre-fix behaviour). The prowess lockstep's mutation test flips it to
// prove the pool reaches a recipient with two grants.
var DebugProwessOnce bool
