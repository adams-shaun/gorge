package kgen

// Flashback costs with a non-mana part (CR 702.34a): gorge
// flashbackCost parses the keyword parameter as a whole cost (ParseCost), so
// "Sac<3/Creature>" (Dread Return), "Sac<1/Mountain>" (Lava Dart) and
// "tapXType<1/Creature.White/white creature>" (Prismatic Strands) are paid
// through the ordinary sacrifice and tap-creatures cost stages of the
// flashback cast. The mana part (none here) is CardDef.AltCost and the rest
// CardDef.FbCost, which the kernel's flashback cast picks in place of the
// spell's additional cost. Only one counted part, a sacrifice, a tap of
// other creatures or a discard, is lowered; a Behold<N/Type> beside the mana is the
// card's behold part (FbBeholdN/FbBeholdF), not an FbCost. A hybrid pip in
// the mana part is announced like a printed cost's. A plain Discard<N/Card>
// is the counted part too.
func (lc *lowerCtx) flashbackCost(param string) error {
	lc.beholdOK = true
	c, err := lc.cost(param)
	lc.beholdOK = false
	if err != nil {
		return err
	}
	mana := c.Mana
	if mana.None {
		mana = manaCost{}
	}
	if mana.X || len(mana.Twobrid) != 0 {
		return refuse("flashback mana part %s", mana)
	}
	if c.BeholdN != 0 {
		// a behold part beside the mana (Kindle the Inner Flame): the
		// card's behold fields, asked in the flashback cast's cost-pick
		// stage (kernel act_behold.go, CardDef.FbBehold)
		rest := c
		rest.Mana, rest.BeholdN, rest.BeholdF = manaCost{}, 0, nil
		if !rest.pureMana() {
			return refuse("flashback cost parts beside a behold")
		}
		lc.out.Flashback = &mana
		lc.out.FbBeholdN, lc.out.FbBeholdF = c.BeholdN, c.BeholdF
		return nil
	}
	rest := c
	rest.Mana = manaCost{}
	counted := 0
	if rest.Sac != nil {
		counted++
	}
	if rest.TapN > 0 {
		counted++
	}
	if rest.DiscardN > 0 && rest.DiscardSpec == "" && !rest.DiscardSelf && !rest.DiscardHand && rest.DiscardF == nil {
		// Discard<N/Card> (any cards from hand): the ordinary discard cost
		// stage of the flashback cast (Twinned Vision)
		counted++
		rest.DiscardN = 0
	}
	rest.Sac, rest.SacN, rest.TapN, rest.TapF = nil, 0, 0, nil
	if counted != 1 || !rest.pureMana() {
		return refuse("flashback cost parts other than one sacrifice or tap-creatures part")
	}
	c.Mana = manaCost{None: true}
	lc.out.Flashback = &mana
	lc.out.FbCost = &c
	return nil
}
