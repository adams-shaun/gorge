package kgen

// K:Kicker with a non-mana part (Vayne's Treachery's
// Sac<1/Creature;Artifact>, Chocobo Kick's Return<1/Land>, Phyrexian
// Scuta's PayLife<3>, Dralnu's Pet's {2}{B} and Discard<1/Creature>). gorge's
// kickerCost is ParseCost(param), a full Cost the kicked cast adds to its
// own (rules cast_begin.go: cost = cost.Plus(kc)), so the part is paid with
// the cast's other cost parts and the kicked mode is offered only when it has
// a candidate (legal_walk_hand.go offerCastable). The kernel keeps the mana
// in CardDef.Kicker and the one non-mana part in CardDef.KickerCost; pays
// that on a kicked cast (kw_kicker.go). A part kop.Cost cannot carry (a
// reveal, or two parts) is a cost program instead (lower_costprog.go).

// kickerNonMana splits a kicker parameter into the mana part and the one
// non-mana part kop.Cost can carry alone; false when the parameter is not a
// single such part (a second part, a reveal, an X kicker).
func (lc *lowerCtx) kickerNonMana(param string) (manaCost, lCost, bool) {
	c, err := lc.cost(param)
	if err != nil || c.XMin != 0 || c.Mana.X || c.RevealN != 0 || c.RevealF != nil {
		return manaCost{}, lCost{}, false
	}
	base := c
	base.Mana = manaCost{None: true}
	if base.pureMana() {
		return manaCost{}, lCost{}, false // no non-mana part
	}
	var found lCost
	n := 0
	try := func(zero func(*lCost)) {
		r := base
		zero(&r)
		if r.pureMana() {
			found = base
			n++
		}
	}
	try(func(r *lCost) { r.Sac, r.SacN = nil, 0 })
	try(func(r *lCost) { r.Return, r.ReturnN = nil, 0 })
	try(func(r *lCost) { r.DiscardN, r.DiscardSpec, r.DiscardF = 0, "", nil })
	try(func(r *lCost) { r.TapN, r.TapF = 0, nil })
	try(func(r *lCost) { r.PayLife = 0 })
	try(func(r *lCost) { r.Forage = nil })
	if n != 1 {
		return manaCost{}, lCost{}, false
	}
	return c.Mana, found, true
}
