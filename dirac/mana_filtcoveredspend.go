package dirac

// A covered funding source whose every mana ability is spend-restricted away
// from the searched play is a dead funding branch (gorge's
// scriptSearch.coveredSpendable, CR 106.6): tapping it floats only mana the
// play refuses and spends one node of the bounded search (Myr Reservoir's
// "Spend this mana only to cast Myr spells or activate abilities of Myr"
// beside Tribute to the World Tree). Only a plain cast of the printed cost
// (from the hand, or under a may-play permission that leaves the cost alone)
// is pruned, a creature is never (it may still be tapped as a cost: convoke,
// crew), and a restricted source still funds a census gap's activation fee
// when that activation admits its mana (Vhal's mana paying School of the
// Unseen's {2}).

// coveredDead reports whether funding source sl is such a dead branch for the
// search's play.
func (fs *filtSearch) coveredDead(k *prep, t *State, sl Slot) bool {
	a := fs.a
	if a.Kind() != AkCast || a.Mode()&^castMayPlay != 0 || t.isCreature(k, sl) {
		return false
	}
	d := t.grantPayDesc(k, a)
	if t.gapSpendable(k, sl, &d) {
		return false
	}
	for m := fs.gaps; m.any(); m = m.rest() {
		gid := m.first()
		if gid == sl {
			continue
		}
		gd := payDesc{class: pdAct, card: t.cardAt(gid), src: gid}
		if t.gapSpendable(k, sl, &gd) {
			return false
		}
	}
	return true
}
