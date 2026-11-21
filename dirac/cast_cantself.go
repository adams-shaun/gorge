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

// DebugCantSorceryOff is the mutation switch (dirac/oracle
// TestEffectflashMutation): an OnlySorcerySpeed$ CantBeCast never lets its
// caster cast at sorcery speed, so the lockstep must diverge.
var DebugCantSorceryOff bool

// castForbidden reports whether a StatCantBeCast forbids player p casting
// card c (object o) -- gorge castRestricted: an active static (a
// battlefield permanent's, or an emblem's) whose caster relation (Caster$:
// StRaiseYou its controller, StRaiseOpp its opponents, neither anyone),
// condition and spell filter (read with the static's controller as "you",
// the card as on the stack) all hold.
//
//go:noinline
func (s *State) castForbidden(k *prep, p uint8, c CardID, o Obj) bool {
	return s.castForbiddenCost(k, p, c, o, nil, nil)
}

// castForbiddenCost is castForbidden for a cast that pays cost (nil: a route
// that announces no X, or the CR 601.2e recheck). cost is the cast's own cost
// and av the offer's availability census (nil: price a fresh one). The X
// look-ahead (castXEscapes) reads cost's {X} exactly as gorge's
// castRestrictedCost does: only a cost carrying {X} can announce one (CR
// 107.3b), and only a payable announcement escapes the restriction.
func (s *State) castForbiddenCost(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, av *avail) bool {
	if k.gen.anySpellCond && s.spellCondFails(k, p, c, o) {
		return true // cast_spellcondition.go
	}
	hit := s.castRestrictionForbids(k, p, c, o, cost, av, false)
	if !hit {
		hit = s.castForbiddenSelf(k, p, c, o) // cast_cantself.go
	}
	return hit
}

// castRestrictionForbids scans the battlefield and emblem CantBeCast statics
// (gorge castRestrictionSources) for one matching c and p. mvOnly keeps only
// a static whose filter reads a mana value: the CR 601.2e recheck needs just
// those, so it does not re-answer a cast-provenance or named-card gate at a
// point gorge does not.
func (s *State) castRestrictionForbids(k *prep, p uint8, c CardID, o Obj, cost *kop.ManaCost, av *avail, mvOnly bool) bool {
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
		if mvOnly && !filterReadsMV(k, st.Filter) {
			return true
		}
		if st.B&kop.CantCastSorcery != 0 && !DebugCantSorceryOff && s.sorcerySpeed(p) {
			return true // OnlySorcerySpeed$: the caster could cast a sorcery, so it does not bind (gorge castSorceryTimingExempt)
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
		if !s.matchSpellCardOffer(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl, stc.emb) {
			return true
		}
		if !mvOnly && s.castXEscapes(k, st.Filter, c, p, ctx.ctrl, o, ctx.srcSl, stc.emb, ctx, cost, av) {
			return true // an announced X the payment can settle lifts the restriction (CR 601.3a)
		}
		if DebugCantCastNamedCount && st.B&kop.CantCastNamed != 0 {
			CantCastNamedHeld++
		}
		hit = true
		return false
	})
	return hit
}

// castForbiddenFree is castForbidden for a cast a resolving spell or ability
// makes (a free cast, a hideaway play, a transformed battle): gorge's stack
// still holds the resolving object there, so no sorcery window is open and an
// OnlySorcerySpeed$ prohibition binds (castSorceryTimingExempt).
func (s *State) castForbiddenFree(k *prep, p uint8, c CardID, o Obj) bool {
	s.cfgx |= cfgxNoSorcery
	f := s.castForbidden(k, p, c, o)
	s.cfgx &^= cfgxNoSorcery
	return f
}

// a card's own "you can't cast this spell unless/during
// ..." (a CantBeCast static on the card itself that works from every zone;
// Jace Reawakened, Proft). No State field: the static lives on the card's own
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

// DebugCantCastNoXLookahead is TestBriselaCantBeCastMutation's switch: the
// cast offer never runs the CR 601.3a X look-ahead, so an X spell its mana
// value lifts out of a CantBeCast bound only at a payable X is withheld and
// the lockstep must diverge.
var DebugCantCastNoXLookahead bool

// filterReadsMV reports whether f's Or chain carries a constant mana-value
// comparison: the one shape the offer's X look-ahead can re-answer for a
// different announced X. A dynamic bound (PredCMCLEV) has no fixed X to
// compare against and is refused in kgen, so it never takes the look-ahead.
func filterReadsMV(k *prep, f kop.FilterID) bool {
	for f != 0 {
		fl := k.t.Filters[f]
		switch fl.Pred {
		case kop.PredCMCLE, kop.PredCMCGE, kop.PredCMCEQ, kop.PredTypesAllCMCLE:
			return true
		case kop.PredProg:
			if id := fl.PredN; id > 0 && int(id) < len(k.t.FilterProgs) {
				for _, l := range k.t.FilterProgs[id].Lits {
					if l.W == kop.FwCMCCmp && l.V.K == kop.VConst {
						return true
					}
				}
			}
		}
		f = fl.Or
	}
	return false
}

// castXEscapes is the offer's CR 601.3a look-ahead, gorge's
// xAnnouncementEscapes: when the restriction matches c at its current mana
// value (X = 0 off the stack, CR 202.3e) but a PAYABLE announced X gives it a
// mana value the restriction no longer matches, the restriction does not bind
// the offer -- the caster may announce the escaping X, and the CR 601.2e
// recheck (castXRestrictionRecheck) then judges the proposal at that X.
//
// Only a cost carrying {X} can announce one (CR 107.3b), so a free or
// alternative cast without {X} never runs this. The candidate announcements
// are the ones av.maxX prices -- the same availability the offer's own
// affordability gate reads -- so an X the payment cannot settle does not lift
// the restriction.
func (s *State) castXEscapes(k *prep, f kop.FilterID, c CardID, p, ctrl uint8, o Obj, src Slot, emb uint8, ctx evalCtx, cost *kop.ManaCost, av *avail) bool {
	if DebugCantCastNoXLookahead || cost == nil || cost.X == 0 || o == NoObj {
		return false
	}
	if s.objs[o].zone == kop.ZoneStack {
		return false // the recheck reads the announced X from the stack entry
	}
	if !filterReadsMV(k, f) {
		return false
	}
	var local avail
	if av == nil {
		s.availFor(k, p, &local)
		av = &local
	}
	mx := av.maxX(cost, pipMaskOf(cost))
	minX := 1
	if ab := spellAb(k, c); ab != 255 && int(ab) < len(k.t.Cards[c].Abilities) {
		if floor := int(k.t.Cards[c].Abilities[ab].Cost.XMin); floor > minX {
			minX = floor
		}
	}
	for x := minX; x <= mx; x++ {
		ctx.xmv = x * int(cost.X)
		if !s.matchSpellCardOfferCtx(k, f, c, p, ctrl, o, src, emb, &ctx) {
			return true
		}
	}
	return false
}

// castXRestrictionRecheck implements CR 601.2e for a mana-value CantBeCast
// restriction once the {X} announcement is made: the spell is on the stack, so
// its mana value counts the announced X (CR 202.3e), and the offer's
// look-ahead no longer applies. It scans only the filters that read a mana
// value, exactly the class the offer's look-ahead can have lifted.
func (s *State) castXRestrictionRecheck(k *prep, e *stackEntry) bool {
	if k.gen == nil || !k.gen.anyCantCast || e.kind != seSpell || k.cards[e.card].cost.X == 0 {
		return false
	}
	return s.castRestrictionForbids(k, e.ctrl, e.card, e.obj, nil, nil, true)
}

// manaStage is the bsMana stage's entry: the CR 601.2e recheck runs before the
// payment window opens, then the ordinary mana step. Kept out of manaStep so
// the hot function grows no branch (codeshape).
func (s *State) manaStage(k *prep) bool {
	if s.castXRestrictionRecheck(k, &s.bld().e) {
		s.reverseCast(k) // CR 601.2e: the announced X makes the CantBeCast restriction apply
		return true
	}
	return s.manaStep(k)
}
