package kgen

// spellReturnOnly reports a spell's Cost$ is its printed mana plus exactly
// one Return<N/Spec> part, on an instant or sorcery (Familiar's Ruse: "As an
// additional cost to cast this spell, return a creature you control to its
// owner's hand"). The part rides the AbSpell ability's cost like any other
// instant additional cost, picked by the cast's pick stages and paid after the
// mana (returnPickOptions, payReturnPicks: gorge payCast's returns before the
// sacrifices); any second pick part stays refused (one pick cost per cast).
func (lc *lowerCtx) spellReturnOnly(c *lCost) bool {
	if c.Return == nil || c.ReturnN < 1 || lc.out.Types&(tInstant|tSorcery) == 0 {
		return false
	}
	rest := *c
	rest.Return, rest.ReturnN = nil, 0
	rest.Mana = manaCost{None: true}
	return rest.pureMana()
}
