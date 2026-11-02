package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Self flash: a card's own `S:Mode$ CastWithFlash | ValidCard$
// Card.Self | EffectZone$ All` ("you may cast this spell as though it had
// flash if ..."; Illusion Spinners, Bard's Company, Serpent of the Pass).
// gorge castWithFlashTargets reads the battlefield statics plus the card's own
// face statics of the mode that name an EffectZone$ (withSelfStatics, CR
// 113.6e), so the carrier grants itself flash from hand once its
// IsPresent$ / PresentCompare$ gate holds for the caster. The gate is read
// with no battlefield source (staticTimingGate: Faerie.YouCtrl on the
// caster's battlefield, Lesson.YouOwn in the caster's graveyard), which is
// condEval with src = NoObj: kgen lowers only gates that never read the
// source (selfFlashShape). The same ability on the battlefield grants nothing
// (castStatic skips a SelfFlash card's own grant).

// DebugSelfFlashOff is the mutation switch (dirac/oracle
// TestSelfflashMutation): selfFlash reads false, so the lockstep must
// diverge.
var DebugSelfFlashOff bool

// selfFlash reports whether card c, cast by p, grants itself flash.
func (s *State) selfFlash(k *prep, p uint8, c CardID) bool {
	if DebugSelfFlashOff {
		return false
	}
	cd := &k.t.Cards[c]
	if !cd.SelfFlash {
		return false
	}
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCastWithFlash && s.matchCard(k, a.Static.Filter, c, p, p) &&
			(a.Static.Cond2 == 0 || s.condEval(k, a.Static.Cond2, evalCtx{src: NoObj, srcSl: NoSlot, ctrl: p})) {
			return true
		}
	}
	return false
}

// castFlash reports whether p may cast card c as though it had flash: a
// battlefield grant (castStatic) or the card's own self grant. gorge reads
// the one castWithFlash(p, id) for every offer. It inlines to the two guards
// (a battlefield static in play, the table has a SelfFlash card) and one
// call, so a table without either pays the guard tests only.
func (s *State) castFlash(k *prep, p uint8, c CardID) bool {
	return (s.statM.any() || k.anySelfFlash) && s.castFlashAny(k, p, c)
}

// castFlashAny is castFlash's cold body.
func (s *State) castFlashAny(k *prep, p uint8, c CardID) bool {
	return s.statM.any() && s.castStatic(k, kop.StatCastWithFlash, p, c) || k.anySelfFlash && s.selfFlash(k, p, c)
}
