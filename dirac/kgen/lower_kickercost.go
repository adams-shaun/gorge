package kgen

// K:Kicker with a non-mana cost (Vayne's Treachery's
// Sac<1/Creature;Artifact>, Chocobo Kick's Return<1/Land>). gorge's
// kickerCost is ParseCost(param), a full Cost the kicked cast adds to its
// own (rules cast_begin.go: cost = cost.Plus(kc)), so a sacrifice or return
// part is paid with the cast's other cost parts and the kicked mode is
// offered only when it has a candidate (legal_walk_hand.go offerCastable).
// The kernel keeps a zero-mana Kicker (the kicked cast option) and pays
// CardDef.KickerCost on a kicked cast (kw_kicker.go).

// kickerNonMana lowers a kicker parameter that is exactly one Sac<1/..> or
// one Return<1/..> part with no mana; anything else reports false and the
// caller refuses it by name.
func (lc *lowerCtx) kickerNonMana(param string) (*lCost, bool) {
	c, err := lc.cost(param)
	if err != nil || !c.Mana.None {
		return nil, false
	}
	sac, ret := c.Sac != nil && c.SacN == 1, c.Return != nil && c.ReturnN == 1
	rest := c
	rest.Sac, rest.SacN, rest.Return, rest.ReturnN = nil, 0, nil, 0
	if sac == ret || !rest.pureMana() || !rest.Mana.None {
		return nil, false
	}
	return &c, true
}
