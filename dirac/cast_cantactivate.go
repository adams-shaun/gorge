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

// prepCantAct arms genPrep.anyCantAct when some card has a StatCantActivate,
// and marks each such card in pcard.cantAct: actForbiddenBy skips the rest.
func prepCantAct(t *kop.Table, p *prep, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantActivate {
				p.cards[i].cantAct = true
				break
			}
		}
	}
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

// eachCantActStatic is actForbiddenBy's eachStatic: it walks only a
// permanent whose card prints a StatCantActivate (pcard.cantAct) and every
// emblem (which has no card), in eachStatic's order. A card without the
// static has no ability the callback accepts, so the verdict is unchanged.
func (s *State) eachCantActStatic(k *prep, f func(st statSrc) bool) {
	for m := s.statM; m.any(); m = m.rest() {
		sl := m.first()
		c := s.cardAt(sl)
		if !k.cards[c].cantAct {
			continue
		}
		lost := s.genLost(k, sl)
		p := &s.perms[sl]
		cd := &k.t.Cards[c]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if lost && !s.ovGateKeptOnLoss(k, sl, a) {
				continue
			}
			if a.Kind == kop.AbStatic || a.Kind == kop.AbReplacement {
				if !f(statSrc{a: a, sl: sl, ctrl: p.ctrl, obj: p.obj}) {
					return
				}
			}
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		lost := s.out(pp)
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			if lost && !e.UntilNextTurn {
				continue
			}
			sl, obj := NoSlot, NoObj
			if e.HostBound() {
				if obj = s.embObj(pp, i); s.objs[obj].zone != kop.ZoneBattlefield {
					continue
				}
				sl = s.objs[obj].slot
			}
			for j := range e.Abilities {
				a := &e.Abilities[j]
				if a.Kind == kop.AbStatic {
					if !f(statSrc{a: a, sl: sl, ctrl: pp, obj: obj, emb: pp<<4 | uint8(i+1)}) {
						return
					}
				}
			}
		}
	}
}

// actForbiddenBy is actForbiddenAb for the player p activating (the
// permanent's controller, or another player of an any-player ability:
// cast_anyactivator.go).
func (s *State) actForbiddenBy(k *prep, sl Slot, p uint8, mana, loyal bool) bool {
	if !k.gen.anyCantAct {
		return false
	}
	if mana && DebugCantActMana {
		return false
	}
	hit, cond := false, false
	s.eachCantActStatic(k, func(stc statSrc) bool {
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
			if c, ok := s.chosenNameOf(stc.sl); !ok || s.nameCardAt(sl) != s.nameKey(c) {
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
