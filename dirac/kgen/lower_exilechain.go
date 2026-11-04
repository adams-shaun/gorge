package kgen

// exileChainSplit splits a printed trigger's own "{N} and ExileFromGrave<1/Spec>"
// cost (Sylvia Tilly, Joy of Discovery) into the exile half, a second IfPaid
// instruction that follows the mana half's pay or decline election, and the
// mana-only cost payInstr lowers as usual. gorge's triggered-cost window
// (rules/cumulative.go) asks pay or decline once, poses the "exile_cost" pick
// over the payer's graveyard (unasked with exactly one candidate), then charges
// the mana half. The kernel's PayMana holds the election and the charge; the
// chained PayExileGrave holds the pick (kop.PayBSubChain, cost_exilepool.go).
// Only the one-card, graveyard-only, not-the-source shape.
func (lc *lowerCtx) exileChainSplit(c lCost) (*lInstr, lCost, error) {
	if c.ExileGN != 1 || c.ExileGBf {
		return nil, c, refuse("resolution mana-and-exile cost of %d cards", c.ExileGN)
	}
	for _, cl := range c.ExileG.Clauses {
		if cl.Flags&fSelf != 0 {
			return nil, c, refuse("resolution exile-from-graveyard cost of the source")
		}
	}
	in := &lInstr{Op: "IfPaid", Obj: lRef{Kind: rYou}, CS: "ExileGraveChain", A: cst(1), F: c.ExileG,
		Note: "exile half of the mana-and-exile cost"}
	c.ExileG, c.ExileGN = nil, 0
	return in, c, nil
}
