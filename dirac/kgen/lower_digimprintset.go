package kgen

// lower_digimprintset.go: Dig's Imprint$ True on a permanent's ability (Reno
// and Rude, Rotisserie Elemental): "exile the top card(s) of a library, then
// you may play the exiled cards this turn". gorge effDig imprints each
// nontoken card the dig moved on the source; a later Effect of the same chain
// remembers Imprinted and grants the play, and the chain's Cleanup clears
// Imprinted. The kernel holds the set for the resolution only
// (genState.imp): the Dig is followed by the kop.ExtDigImprint marker, the
// Effect's MayPlay reads kop.RefImprinted and the Cleanup runs
// kop.ExtClearImprinted. That is exact only while the whole read happens in
// the one resolution, so the shape is narrow: every card of the window is
// taken to exile (ChangeNum$ All, nothing to choose), and Defined$ Imprinted
// outside the chain still refuses.

// digImprintPermanent vets the Dig and opens the set for the current chain.
func (lc *lowerCtx) digImprintPermanent(ps *params, dest int, take lValue) error {
	if dest != zExile || !take.isConst() || take.N != -1 {
		return refuse("Dig Imprint$ on a permanent outside a take-all to exile")
	}
	for _, k := range []string{"ChangeValid", "Optional", "ExileFaceDown", "WithMayLook", "RememberChanged", "Reveal", "ForceRevealToController"} {
		if _, ok := ps.m[k]; ok {
			return refuse("Dig Imprint$ on a permanent beside %s$", k)
		}
	}
	if lc.chainRoot == nil || lc.digImprintRoot != nil && lc.digImprintRoot != lc.chainRoot {
		return refuse("Dig Imprint$ on a permanent outside one chain")
	}
	lc.digImprintRoot = lc.chainRoot
	return nil
}

// digImprintSetLive reports whether the chain being lowered is the one whose
// Dig imprinted the resolution's set.
func (lc *lowerCtx) digImprintSetLive() bool {
	return lc.digImprintRoot != nil && lc.digImprintRoot == lc.chainRoot
}
