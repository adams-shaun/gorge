package dirac

// ctrKey folds into key the counters of the payer's counter-cost sources: gorge
// dedups its prefix search on every own permanent's tapped bit and counters
// (scriptStateKey), so a Vivid Meadow tapped for its plain {W} and one tapped
// for {W} by removing a charge counter are two states, and the second is
// searched. Only a counter-cost mana ability's own counter kind changes along
// the path, and it can change on a source the census covers as well as on a
// gap, so every own permanent with one is folded (bf), not only the gaps.
func (t *State) ctrKey(k *prep, bf slotMask, key *filtKey) {
	if len(k.ctrAb) == 0 {
		return
	}
	i := 0
	for b := bf; b.any(); b = b.rest() {
		sl := b.first()
		a := k.ctrAbOf(t.cardAt(sl))
		if a == nil {
			continue
		}
		key.ctr[i&7] += uint16(t.perms[sl].ctrN(t, int(a.Cost.Ctr)))
		i++
	}
}
