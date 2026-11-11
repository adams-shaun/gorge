package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugCantCastNamedCount turns on CantCastNamedHeld, the count of casts a
// named CantBeCast static (Alhammarret, High Arbiter) withheld; oracle tests only.
var (
	DebugCantCastNamedCount bool
	CantCastNamedHeld       int
)

// castForbidden reports whether a StatCantBeCast forbids player p casting
// card c (object o) -- gorge castRestricted: an active static (a
// battlefield permanent's, or an emblem's) whose caster relation (Caster$:
// StRaiseYou its controller, StRaiseOpp its opponents, neither anyone),
// condition and spell filter (read with the static's controller as "you",
// the card as on the stack) all hold.
//
//go:noinline
func (s *State) castForbidden(k *prep, p uint8, c CardID, o Obj) bool {
	if k.gen.anySpellCond && s.spellCondFails(k, p, c, o) {
		return true // cast_spellcondition.go
	}
	hit := false
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantBeCast || a.Static.Zone == kop.ZoneCommand {
			return true // a self static is read by castForbiddenSelf
		}
		st := &a.Static
		if st.StFlags&kop.StRaiseYou != 0 && stc.ctrl != p || st.StFlags&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		if st.B&kop.CantCastNamed != 0 {
			// Alhammarret, High Arbiter: only the card the source named as it entered
			if n, ok := s.chosenNameOf(stc.sl); !ok || n != c {
				return true
			}
		}
		ctx := s.ctxStatic(stc)
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, ctx) {
			return true
		}
		if st.A > 0 && s.castLimitOpen(st, p) {
			return true // static_castlimit.go: the cap does not bind yet
		}
		if s.matchSpellCardOffer(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl) {
			if DebugCantCastNamedCount && st.B&kop.CantCastNamed != 0 {
				CantCastNamedHeld++
			}
			hit = true
			return false
		}
		return true
	})
	if !hit {
		hit = s.castForbiddenSelf(k, p, c, o) // cast_cantself.go
	}
	return hit
}

// a card's own "you can't cast this spell unless/during
// ..." (S:Mode$ CantBeCast | ValidCard$ Card.Self | EffectZone$ All; Jace
// Reawakened, Proft). No State field: the static lives on the card's own
// abilities and is read at the cast offer.

// DebugCantSelfOff is TestCantselfMutation's switch: self CantBeCast
// statics are ignored.
var DebugCantSelfOff bool

// castForbiddenSelf: card c's own CantBeCast statics (EffectZone$ All,
// Zone == ZoneCommand; gorge castRestrictionSources) forbid p casting it:
// StCasterActive needs p to be the active player, Cond2 is read with the
// card's owner as "you" (gorge: a card off the battlefield is controlled
// by its owner; o == NoObj in unit tests: p).
//
//go:noinline
func (s *State) castForbiddenSelf(k *prep, p uint8, c CardID, o Obj) bool {
	if DebugCantSelfOff {
		return false
	}
	ctrl := p
	if o != NoObj {
		ctrl = s.objs[o].owner()
	}
	cd := &k.t.Cards[c]
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantBeCast || a.Static.Zone != kop.ZoneCommand {
			continue
		}
		st := &a.Static
		if st.StFlags&kop.StCasterActive != 0 && p != s.active {
			continue
		}
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, evalCtx{src: o, srcSl: NoSlot, ctrl: ctrl}) {
			continue
		}
		return true
	}
	return false
}
