package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The subsume run (gorge rules/potential_script.go potentialPrefixScript's
// third search, subsumeAnswers). Where gorge's plain prefix search ran out of
// its node budget (scriptSearch.limited) with no fee-carrying gap to fund
// (census.fees() == 0) and the play's base cost is coloured pips alone
// (scriptNeedCounts), gorge searches again in the plain order with every
// colour ask narrowed to the colours the floating pool still lacks of that
// cost: a covered colour pays nothing more, so each line through it is
// dominated by the same line through a lacking one (CR 601.2g-h, 605.1a).
// Wakeroot Elemental's {G}{G}{G}{G}{G} beside two Forests and four Islands
// under Dune Chanter's "{T}: Add one mana of any color": the plain search
// spends its 160 nodes on the Islands' printed {U} and the grants' other
// colours; the subsume run answers each grant's colour ask {G} and finds three
// Islands' green before the Forests pay the rest.
//
// The narrowing reads gorge's colour asks whose every option names a mana
// symbol (scriptManaUnits) and that have more than one answer: a grant's or a
// lone ability's colour ask (branchSel, sacBranch and costBranch on an ask
// of a named mask). The choose-a-mana-ability ask beside a single-colour
// ability names no symbol for that option, so it stays whole.

// grantSubsume runs the subsume search after a failed plain grantSearch run g
// (g holds that run's result): false, with g's stall kept, when gorge's
// conditions for the run do not hold or it finds no prefix either.
func (s *State) grantSubsume(k *prep, p uint8, cost *kop.ManaCost, ex Slot, d payDesc, need [kop.NumManaKinds]bool, g *grantSrch) bool {
	if !g.limited || g.funding != 0 || !d.hasPlay {
		return false
	}
	cnt, ok := s.playCostCounts(k, d.play)
	if !ok {
		return false
	}
	stall := g.stall
	if s.grantSearchRun(k, p, cost, ex, d, need, g, &cnt, false) {
		return true
	}
	if stall != 0 {
		g.stall = stall // the plain run's first stall stays the one named
	}
	return false
}

// playCostCounts is gorge's scriptNeedCounts: play a's base cost's coloured
// pips per colour (playBaseCost), ok false when that cost has a part any
// colour may help pay (a generic, {X}, snow, hybrid, Phyrexian or twobrid
// pip) or a forage, or has no coloured pip at all.
func (s *State) playCostCounts(k *prep, a Action) (cnt [kop.NumManaKinds]uint8, ok bool) {
	m, f, base := s.playBaseCost(k, a)
	if !base || m.Generic != 0 || m.X != 0 || m.Snow != 0 || m.Hyb[0] != 0 || m.Tb != 0 || f&kop.CostForage != 0 {
		return cnt, false
	}
	for c, n := range m.Pips {
		if n > 0 {
			cnt[c], ok = n, true
		}
	}
	return cnt, ok
}

// subsumeCols is gorge's subsumeAnswers on a colour ask (ask, its options all
// naming a symbol: named) of cols, in the subsume run: only the colours the
// pool (its unspendable units too, which gorge's pool counts) holds fewer of
// than the cost asks. An ask of one answer, and every ask of the plain run,
// keeps cols whole; none kept leaves the line no answer.
func (g *grantSrch) subsumeCols(t *State, cols []uint8, ask, named bool) []uint8 {
	if !g.subsume || !ask || !named || len(cols) <= 1 {
		return cols
	}
	pool := &t.pl[SeatIx(g.p)].pool
	out := cols[:0]
	for _, c := range cols {
		if g.costCol[c] > pool[c]+g.ghost[c] {
			out = append(out, c)
		}
	}
	return out
}

// subsumeKeeps is subsumeCols' keep-rule for one enumerated answer c of a
// multi-answer colour ask the search's gap branches walk themselves (the
// counter-cost, wide and Taps-deferred ones): the subsume run keeps only the
// colours the pool, its unspendable units too, holds fewer of than the cost
// asks. The read is the ask's pool, before the step's mana floats in, as
// branchSel's is; the branches apply it after charging the ask's node, and
// every ask of the plain run keeps its answers whole.
func (g *grantSrch) subsumeKeeps(t *State, c uint8) bool {
	return g.costCol[c] > t.pl[SeatIx(g.p)].pool[c]+g.ghost[c]
}
