package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// flashXCap is gorge's xAsk cap for a `ValidSA$ Spell.XCostLE<N>` self flash
// grant (Silver Scrutiny; rules/flash_xcost_cap.go flashXCap): an ordinary
// cast announced at a time a sorcery could not have been cast, whose timing
// rests on that grant alone, may announce only X <= N, because CR 601.2e's
// recheck reverses any larger X. kgen stores 1+N in the SelfFlash carrier's
// own StatCastWithFlash Static.A. ok is false when no cap applies: a
// sorcery-timed cast, a Play effect's cast (gorge's offSorcery is set only
// by beginCast), an instant or Flash card, or a battlefield or emblem grant
// that also covers the cast.
func (s *State) flashXCap(k *prep, e *stackEntry) (n int, ok bool) {
	if e.kind != seSpell || !k.anySelfFlash {
		return 0, false
	}
	cd := &k.t.Cards[e.card]
	if !cd.SelfFlash || k.cards[e.card].instant {
		return 0, false
	}
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCastWithFlash && a.Static.A > 0 {
			n, ok = int(a.Static.A)-1, true
		}
	}
	if !ok || s.freePlayCast() || s.preCastSorcery(e.ctrl) {
		return 0, false
	}
	if s.statM.any() && k.anyFlashStat && s.castStatic(k, kop.StatCastWithFlash, e.ctrl, e.card) || k.anyEmbFlash && s.embFlash(k, e.ctrl, e.card) {
		return 0, false
	}
	return n, true
}

// preCastSorcery is sorcerySpeed as the board stood before the build pushed
// its spell (CR 601.2a): gorge captures offSorcery in beginCast, before the
// push, so the spell's own stack entry does not count.
func (s *State) preCastSorcery(p uint8) bool {
	sp := s.sp
	if s.bld().bfl&bflPushed != 0 && sp > 0 {
		sp--
	}
	return p == s.active && (s.step == kop.StepMain1 || s.step == kop.StepMain2) && sp == 0 && s.cfgx&cfgxNoSorcery == 0
}
