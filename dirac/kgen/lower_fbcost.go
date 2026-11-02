package kgen

// Flashback costs with a non-mana part (CR 702.34a): gorge
// flashbackCost parses the keyword parameter as a whole cost (ParseCost), so
// "Sac<3/Creature>" (Dread Return), "Sac<1/Mountain>" (Lava Dart) and
// "tapXType<1/Creature.White/white creature>" (Prismatic Strands) are paid
// through the ordinary sacrifice and tap-creatures cost stages of the
// flashback cast. The mana part (none here) is CardDef.AltCost and the rest
// CardDef.FbCost, which the kernel's flashback cast picks in place of the
// spell's additional cost. Only one counted part, a sacrifice or a tap of
// other creatures, is lowered.
func (lc *lowerCtx) flashbackCost(param string) error {
	c, err := lc.cost(param)
	if err != nil {
		return err
	}
	mana := c.Mana
	if mana.None {
		mana = manaCost{}
	}
	if mana.X || len(mana.Hybrid) != 0 || len(mana.Twobrid) != 0 {
		return refuse("flashback mana part %s", mana)
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
	rest.Sac, rest.SacN, rest.TapN, rest.TapF = nil, 0, 0, nil
	if counted != 1 || !rest.pureMana() {
		return refuse("flashback cost parts other than one sacrifice or tap-creatures part")
	}
	c.Mana = manaCost{None: true}
	lc.out.Flashback = &mana
	lc.out.FbCost = &c
	return nil
}
