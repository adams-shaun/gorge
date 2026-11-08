package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// the gorge pin follow 057818e7f -> e276bf2b3. Cold code, out
// of line.
//
// gorge lists one may-play offer per PERMISSION
// (engine/rules/mayplay.go mayPlayPermissions): every untyped grant without
// a MayPlayLimit$ folds into one empty-key offer, listed first; every
// limited (StOncePerTurn) or MayPlayText$-typed static is its own keyed
// offer, and a cast through one stamps its key (cast_commit.go) so its
// once-per-turn count is per static (mayPlayKeyLimitReached). Two offers of
// one card are "ambiguous" to the potential planner (potential_plan.go).

// mayPlayOffers is the number of may-play offers gorge lists for p's card o,
// capped at 2 (Config.GorgeUnpriced's "ambiguous"). Every unlimited static,
// in any zone, folds into the one empty-key offer; every StOncePerTurn
// static (MayPlayLimit$; in the corpus every MayPlayText$-typed static
// carries one, and kgen refuses MayPlayAltManaCost$, the one untyped
// limited shape gorge folds) is its own (Muldrotha's per-type statics,
// Karador's once-a-turn grant beside an unlimited one). Quilled Greatwurm's
// own permission counts only when payable (pair 20520).
func (s *State) mayPlayOffers(k *prep, p uint8, o Obj) int {
	n, fold := 0, false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		fl := st.a.Static.StFlags
		if fl&kop.StRemove6Counters != 0 && s.creatureCounters(k, p) < int(st.a.Static.A) || k.gen.anyMayRaise && !s.mayRaiseOK(k, p, o) {
			return true
		}
		if fl&kop.StOncePerTurn == 0 {
			if fold {
				return true
			}
			fold = true
		}
		n++
		return n < 2
	})
	return n
}

// mayPlayPlannedZone reports whether gorge's payment builder plans an untyped
// may-play cast from z (payment_plan.go plannedMayPlayZone): only the
// graveyard and exile, never a top-of-library or hand permission.
func mayPlayPlannedZone(z kop.Zone) bool {
	return z == kop.ZoneGraveyard || z == kop.ZoneExile
}

// mayPlayFolds reports whether some unlimited static lets p play o: gorge
// then lists the empty-key offer first and the lockstep takes it, so the
// cast spends no limited static's use (mayPlayPermissions).
func (s *State) mayPlayFolds(k *prep, p uint8, o Obj) bool {
	fold := false
	s.mayPlayStatics(k, p, o, func(st statSrc) bool {
		fl := st.a.Static.StFlags
		if fl&kop.StRemove6Counters != 0 && s.creatureCounters(k, p) < int(st.a.Static.A) || k.gen.anyMayRaise && !s.mayRaiseOK(k, p, o) {
			return true
		}
		if fl&kop.StOncePerTurn == 0 {
			fold = true
			return false
		}
		return true
	})
	return fold
}

// mayPlayPrefixPays: gorge's potential planner prices a may-play cast from its
// composed cost (rules potentialPlayVerdictShared's "mayplay" arm,
// potentialModeCastPlan), with none of the shape declines a hand cast meets
// (an optional cost, an exile or collect-evidence part), so a cast its census
// cannot pay takes a prefix search through the sources the census leaves out:
// a tap-another source (Springleaf Drum), a filter (Jeskai Devotee's {1}: any
// colour), a granted ability (Insidious Roots' Plant), a census gap. The
// caller's scripted walks (scriptOffers, grantCap) then hold it to a script
// gorge's search finds (CR 601.2g, 605.3a).
func (s *State) mayPlayPrefixPays(k *prep, p uint8, c CardID, o Obj) bool {
	if k.gen == nil || s.g.availX&(availGaps|availAny) == 0 && len(k.gen.grantPC) == 0 && !s.scriptSourceLive(k, p) {
		return false
	}
	cost := s.featCost(k, p, c, o, k.cards[c].cost)
	if s.g.mayPlayAny {
		cost = anyType(cost)
	}
	return s.affordsFor(k, p, &cost)
}
