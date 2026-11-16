package dirac

// ctrKey folds into key the counters of the search's counter-cost gaps: gorge
// dedups its prefix search on every own permanent's tapped bit and counters
// (scriptStateKey), so a Vivid Meadow tapped for its plain {W} and one tapped
// for {W} by removing a charge counter are two states, and the second is
// searched. Only a counter-cost gap's own counter kind changes along the path.
func (fs *filtSearch) ctrKey(k *prep, t *State, key *filtKey) {
	if len(k.ctrAb) == 0 {
		return
	}
	i := 0
	for b := fs.other; b.any(); b = b.rest() {
		sl := b.first()
		a := k.ctrAbOf(t.cardAt(sl))
		if a == nil {
			continue
		}
		key.ctr[i&3] += uint16(t.perms[sl].ctrN(t, int(a.Cost.Ctr)))
		i++
	}
}
