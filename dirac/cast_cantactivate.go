package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a printed S:Mode$ CantBeActivated (kop.StatCantActivate,
// CR 602.5; gorge abilityRestricted / abilityRestrictionApplies,
// statics_cast.go). A locked permanent's activated abilities are not offered
// (canActivate), and a locked mana ability is no mana source: gorge's
// appendAvailableManaAbilitiesGateCompute gates every mana arm through
// abilityRestricted, so neither its planner nor its payment window sees the
// source (manaPCGen and the planner paths that bypass it).

// DebugCantActMana (test only, TestCantactMutation): actForbidden never
// forbids a mana ability, so a locked mana source stays live.
var DebugCantActMana bool

// DebugCantActCount (test only, TestCantactLockstep) arms the hold-out
// counters below: CantActHeld counts actForbidden's activation-offer hits,
// CantActManaHeld its mana-source hits, CantActCondHeld the hits (either) of
// a conditional static (Grand Abolisher's Condition$ PlayerTurn).
var (
	DebugCantActCount                             bool
	CantActHeld, CantActManaHeld, CantActCondHeld uint64
)

// prepCantAct arms genPrep.anyCantAct when some card has a StatCantActivate.
func prepCantAct(t *kop.Table, g *genPrep) {
	g.anyCantAct = tableHasStatic(t, kop.StatCantActivate) || g.anyCantSac
}

// actForbidden reports whether a StatCantActivate forbids the controller of
// the permanent at sl activating one of its abilities (mana: a mana
// ability) -- gorge abilityRestrictionApplies: the activator relation
// (Activator$: StRaiseYou the static's controller, StRaiseOpp its
// opponents), the condition (Cond2, read as the static's), ValidSA$ (A = 1:
// Activated.!ManaAbility spares mana abilities; A = 2: Activated.Loyalty
// reaches loyalty abilities only) and the card filter (read
// with the static's controller as "you" and its source as the
// EnchantedBy anchor) all hold. eachStatic skips a source that lost its
// abilities, as gorge's activeStatics does.
//
//go:noinline
func (s *State) actForbidden(k *prep, sl Slot, mana bool) bool {
	if mana && k.gen.anyCantSac && s.manaSacBlocked(k, sl) {
		return true // a sacrificing mana ability whose source can't be sacrificed (static_cantsacrifice.go)
	}
	return s.actForbiddenAb(k, sl, mana, false)
}

// actForbiddenAb is actForbidden for one activated ability: loyal says it is
// a planeswalker loyalty ability, the only kind an A = 2 static (ValidSA$
// Activated.Loyalty, The Immortal Sun; gorge activatedMatchesValidSA)
// forbids.
func (s *State) actForbiddenAb(k *prep, sl Slot, mana, loyal bool) bool {
	return s.actForbiddenBy(k, sl, s.perms[sl].ctrl, mana, loyal)
}

// actForbiddenBy is actForbiddenAb for the player p activating (the
// permanent's controller, or another player of an any-player ability:
// legal_anyact.go).
func (s *State) actForbiddenBy(k *prep, sl Slot, p uint8, mana, loyal bool) bool {
	if mana && DebugCantActMana {
		return false
	}
	hit, cond := false, false
	s.eachStatic(k, func(stc statSrc) bool {
		a := stc.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatCantActivate {
			return true
		}
		st := &a.Static
		if st.StFlags&kop.StRaiseYou != 0 && stc.ctrl != p || st.StFlags&kop.StRaiseOpp != 0 && stc.ctrl == p {
			return true
		}
		if mana && st.A&3 == 1 || st.A&3 == 2 && !loyal {
			return true
		}
		if st.A&kop.CantActNamed != 0 {
			// Sorcerous Spyglass: only a source with the static source's chosen name
			if c, ok := s.chosenNameOf(stc.sl); !ok || s.nameCardAt(sl) != c {
				return true
			}
		}
		if st.Cond2 != 0 && !s.condEval(k, st.Cond2, s.ctxStatic(stc)) {
			return true
		}
		if s.matchPermGen(k, st.Filter, sl, stc.ctrl, stc.sl, layerFull) {
			hit, cond = true, st.Cond2 != 0
			return false
		}
		return true
	})
	if hit && DebugCantActCount {
		if mana {
			CantActManaHeld++
		} else {
			CantActHeld++
		}
		if cond {
			CantActCondHeld++
		}
	}
	return hit
}
