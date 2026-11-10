package kgen

// sacrificedIsCost: the ability's Sacrificed$ group is exactly the one
// object a sacrifice paid as its own cast or activation cost, whose last
// known information the kernel snapshots into the entry (snapSacPick).
// The snapshot holds one picked permanent: a Sac<2/...> cost would read
// only the first of gorge's group, so it refuses. A Sac<1/CARDNAME>
// (SacSelf) cost is refused too: the kernel's sacrificed-object snapshot
// is the picked-sacrifice one (snapSacPick), and kgen does not prove the
// self-sacrifice is recorded on every path that can pay it, so the group
// is not read exactly. A trigger's entry LKI is its trigger object's, and
// a resolution cost's sacrifice takes no snapshot, so those refuse
// (C).
func (lc *lowerCtx) sacrificedIsCost() bool {
	switch lc.ab.Kind {
	case akSpell:
		c := lc.out.SpellCost
		return c != nil && c.Sac != nil && c.SacN == 1 && !c.SacSelf && lc.out.AltAddCost == nil
	case akActivated:
		return lc.ab.Cost.Sac != nil && lc.ab.Cost.SacN == 1 && !lc.ab.Cost.SacSelf
	}
	return false
}

// discardedIsCost: the ability is a spell whose own cost discards exactly
// one chosen card (Discard<1/Card>), the one card the kernel records
// (stackEntry.trigObj, RefDiscarded).
func (lc *lowerCtx) discardedIsCost() bool {
	if lc.ab.Kind != akSpell {
		return false
	}
	c := lc.out.SpellCost
	return c != nil && c.DiscardN == 1 && c.DiscardSpec == "" && !c.DiscardSelf && lc.out.AltAddCost == nil
}

// targetMVReadable: Targeted$CardManaCost of a battlefield or graveyard
// target slot, or of exactly one spell on the stack (the kernel's
// targetMV). A spell on the stack adds its announced X to the printed
// total (CR 202.3e; objMV), and once it has left the stack the printed
// total is the whole mana value: Unravel reads its countered target.
func (lc *lowerCtx) targetMVReadable(ref lRef) bool {
	if ref.Kind != rTarget || ref.N < 0 || ref.N >= len(lc.ab.Targets) || ref.N > 2 {
		return false
	}
	t := lc.ab.Targets[ref.N]
	if t.Spell {
		return !t.StackAb && t.Zone == zStack && isConstN(t.Min, 1) && isConstN(t.Max, 1)
	}
	return t.Zone == zBattlefield || t.Zone == zGraveyard
}
