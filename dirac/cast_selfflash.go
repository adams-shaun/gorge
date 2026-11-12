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

// DebugEmbFlashOff and DebugEmbForgetOff are the mutation switches (dirac/oracle
// TestEffectflashMutation): an emblem's spell-flash grant reads false, or a
// cast never ends a ForgetOnCast$ emblem, so the lockstep must diverge.
var DebugEmbFlashOff, DebugEmbForgetOff bool

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
// battlefield grant (castStatic), the card's own self grant, or one of p's
// Effect emblems. gorge reads the one castWithFlash(p, id) for every offer.
// It inlines to the three guards (a battlefield static in play, the table
// has a SelfFlash card, the table has an emblem grant) and one call, so a
// table without any pays the guard tests only.
func (s *State) castFlash(k *prep, p uint8, c CardID) bool {
	return (s.statM.any() || k.anySelfFlash || k.anyEmbFlash) && s.castFlashAny(k, p, c)
}

// castFlashAny is castFlash's cold body.
func (s *State) castFlashAny(k *prep, p uint8, c CardID) bool {
	return s.statM.any() && s.castStatic(k, kop.StatCastWithFlash, p, c) || k.anySelfFlash && s.selfFlash(k, p, c) || k.anyEmbFlash && s.embFlash(k, p, c)
}

// embFlash reports whether one of player p's emblems (an Effect's
// StaticAbilities$ CastWithFlash with Caster$ You: Borne Upon a Wind,
// Alchemist's Refuge, Winding Canyons, Teferi, Time Raveler's +1; gorge
// effectFlashGrants) lets p cast card c as though it had flash. "You" is the
// emblem's player, so only p's own emblems count; the emblem has no source,
// and the card filter is read with p as "you".
func (s *State) embFlash(k *prep, p uint8, c CardID) bool {
	if DebugEmbFlashOff {
		return false
	}
	for i := 0; i < int(s.g.nemb[p]); i++ {
		e := &k.t.Emblems[s.embIdx(p, i)-1]
		for j := range e.Abilities {
			a := &e.Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCastWithFlash && s.matchCard(k, a.Static.Filter, c, p, p) {
				return true
			}
		}
	}
	return false
}

// embFlashForget ends the emblem of any player whose CastWithFlash static names the
// spell just cast in its Filter2 (the Effect's ForgetOnCast$: Quicken's "the
// next sorcery spell you cast this turn"): gorge effectCastSweep drops the
// whole effect at the cast's completion, whether or not the cast used the
// grant. The filter is read with the emblem's player as "you".
//
//go:noinline
func (s *State) embFlashForget(k *prep, e *stackEntry) {
	if DebugEmbForgetOff {
		return
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		var drop uint8
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			em := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range em.Abilities {
				a := &em.Abilities[j]
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCastWithFlash && a.Static.Filter2 != 0 &&
					s.matchCardGen(k, a.Static.Filter2, e.card, e.ctrl, pp, e.obj) {
					drop |= 1 << uint(i)
				}
			}
		}
		if drop != 0 {
			s.embCompact(pp, drop)
		}
	}
}
