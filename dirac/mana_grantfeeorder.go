package dirac

// The grant search's fee-order fallback (gorge potentialPrefixScript's second
// build with scriptSearch.feeOrder, the mirror of mana_filterfeeorder.go for a
// board whose census gaps are granted mana abilities). When the plain run of a
// plain cast's prefix search spends its whole node budget without a script,
// gorge runs it again with the covered funding first, then the census gaps
// whose mana ability costs mana and makes more than one unit (Desolate Mire's
// {1},{T}: Add {W}{B}, beside a granted any-colour tap), then the other gaps.
// A plain walk over a Rimewood Falls (printed {U}, {G}, then the grant's five
// colours), an Island and a Mire spends its 160 nodes on the printed colours'
// subtrees before the grants' {R}s; the reordered walk floats the funding and
// the cheap grants first (Mijae Djinn's {R}{R}{R} under a transformed Mystic
// Skull's "{T}: Add one mana of any color" on every land).

// feeOrderOf reports whether gorge's fee-order run follows the plain run g,
// which found no script (mana_filterfeeorder.go feeOrderFallback): a play at
// the printed cost — a plain cast (mode 0), a mayplay cast (mode castMayPlay,
// the granted from-exile permission's printed-cost cast) or an activation
// (gorge's optAbility; isAbilityPlay is the printed/gained twin cut) — with a
// fee gap among the candidates, outside the phased order's own scope
// (phasedScope), whose plain run's budget ran out (a completed search saw
// every state the reordered one reaches). This is gorge potentialPrefixScript
// build's condition (optCast Mode "" or "mayplay", or optAbility, at the
// alternative-cost index 0): the activation's fee-carrying gap pays its filter
// fees the same way (CR 601.2g), so its budget-spent plain run takes the fee
// order too (Electric Eel beside Vine Trellis and Slagwurm Armor).
func (g *grantSrch) feeOrderOf(s *State) bool {
	if !g.limited || !g.d.hasPlay || g.phasedScope(s) {
		return false
	}
	printed := (g.d.play.Kind() == AkCast && (g.d.play.Mode() == 0 || g.d.play.Mode() == castMayPlay)) ||
		isAbilityPlay(g.d.play)
	if !printed {
		return false
	}
	for i := 0; i < g.gapN; i++ {
		if g.gap[i] && g.feeGap[i] {
			return true // (grantFeeGap marked it when the candidates were built)
		}
	}
	return false
}

// feeOrderPasses is visit's activation walk under the fee order: the covered
// funding, then the fee gaps, then the other gaps (gorge scriptPhaseAt with
// feeOrder). The fee of a fee gap is paid from the floating pool, so the
// funding floats first (CR 601.2b, 605.3b).
func (g *grantSrch) feeOrderPasses(t *State, sacd bool, sacStepped *slotMask) (found, stop bool) {
	if found, stop = g.fundingPass(t); found || stop {
		return found, stop
	}
	for pass := 0; pass < 2; pass++ {
		if pass == 1 && g.printedSacPass(t, sacd, *sacStepped) { // a tapped holder's printed sacrifice (mana_grantcost.go)
			return true, false
		}
		var tried [MaxPerm]Slot
		ntried := 0
		for i := 0; i < g.gapN; i++ {
			if g.feeGap[i] != (pass == 0) {
				continue
			}
			if g.gapTwinSkip(t, i, &tried, &ntried) {
				continue // an indistinguishable earlier gap already walked this subtree (gorge's scriptTwin)
			}
			if found, stop = g.gapVisit(t, i, sacd, sacStepped); found || stop {
				return found, stop
			}
		}
	}
	return false, false
}
