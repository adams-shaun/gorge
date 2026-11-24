package dirac

// planTapsInExtra floats the extra mana a TapsForMana trigger adds when the
// tiered planner's second pass (pk.tapsIn, planTiered0) taps a Taps-deferred
// source: the gated and any-colour StatTapExtra statics the planner's pricing
// readers skip (tapExtraReaches) float at the tap itself, as the prefix
// search's gap taps float them (tapDeferFloat's addExtraCol). gorge's prefix
// search taps the deferred source before the play and resolves its trigger
// there, so the extra floats beside the source's own mana (Buried in the
// Garden on an opponent's Plains: "its controller adds an additional one mana
// of any color", CR 106.12a, 605.1b). The any-colour answer is the first pip
// the cost owes in W U B R G order, else {W} (fillTapAnyColOwed), the answer
// gorge's WindowChoices gives. An ordinary source's tap, and every tap of the
// plain first pass, is unchanged.
func (s *State) planTapsInExtra(k *prep, p uint8, pk *planner, sl Slot) {
	if !pk.tapsIn || k.gen == nil || !k.gen.anyTapExtra || !s.tapsDeferred(k, p, sl) {
		return
	}
	owed := pk.cpips
	s.fillTapAnyColOwed(k, p, sl, &owed)
	s.addGatedExtra(k, p, sl, s.perms[sl].ctrl)
}
