package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// mayParts is what the permissions admitting a card contribute to its
// may-play offer: a zone opened, the mana cost waived, mana of any type.
type mayParts struct{ grant, free, anyMana bool }

// mayPlayParts joins the parts of every permission admitting o into one
// offer: a free-only static (Weftwalking) opens no zone but waives the cost
// of the cast another static grants (CR 118.9). A pay-life static (Gwenom)
// is counted apart in life: gorge offers its alternative beside the plain
// cast another permission grants (AltCostIndex 1 and 0), and one alternative
// per such static (AltCostIndex 1, 2: Gwenom's Effect copied).
func (s *State) mayPlayParts(k *prep, p uint8, o Obj) (rest mayParts, life int) {
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		fl := st.a.Static.StFlags
		if fl&kop.StRemove6Counters != 0 && s.creatureCounters(k, p) < int(st.a.Static.A) {
			return true // the extra cost can't be paid (Quilled Greatwurm)
		}
		if fl&kop.StMayPayLife != 0 {
			if s.payLifeOfferable(k, p, o) {
				life++ // the life alternative, offered apart (cast_staticpaylife.go)
			}
			return true // else the mana value in life can't be paid
		}
		if fl&kop.StFreeOnly == 0 || s.objs[o].zone == kop.ZoneHand {
			rest.grant = true // a free-only static opens no zone; a hand cast is already legal
		}
		rest.free = rest.free || fl&kop.StWithoutManaCost != 0
		rest.anyMana = rest.anyMana || fl&kop.StAnyManaType != 0
		return true
	})
	return rest, life
}

// mayPlayUnpriced reports whether gorge's planner leaves the plain may-play
// cast of o unpriced, so it is listed only when the pool pays.
func (s *State) mayPlayUnpriced(k *prep, p uint8, o Obj, anyMana bool, av *avail) bool {
	// Config.GorgeUnpriced: gorge's planner funds an any-type cast's pips
	// as generic where it plans the shape (Tinybones), and prices no cast
	// two permissions offer (Muldrotha's per-type statics for an artifact
	// creature: "ambiguous"); gorge lists those only when the pool pays
	unpriced := false
	if s.cfg&cfgGorgeUnpriced != 0 {
		if anyMana {
			cost := s.adjustCostGen(k, p, s.objs[o].card, o, k.cards[s.objs[o].card].cost)
			cost = planUnderRider(cost, true, false)                                           // a hybrid pip is generic to the plan too (pay.PlanManaUnderRider)
			unpriced = !s.castAffordsR(k, p, s.objs[o].card, s.restrOriginOfObj(o), &cost, av) // (the restricted sources the planner counts price it too)
		} else {
			// Two offers are ambiguous to gorge's potential planner,
			// but its payment builder still plans the untyped
			// (MayPlayPerm=="") offer from a graveyard or exile
			// (payment_plan.go plannedMayPlayZone), so that cast is
			// priced rather than ambiguous (Storied Veteran beside
			// The Holodeck). A top-of-library cast has no such plan
			// (Boltwave beside Madame Web and Johann).
			unpriced = s.mayPlayOffers(k, p, o) >= 2 &&
				!(s.mayPlayFolds(k, p, o) && mayPlayPlannedZone(s.objs[o].zone))
		}
	}
	return unpriced
}
