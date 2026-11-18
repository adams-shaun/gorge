package oracle

import "testing"

// CR 601.2f with CR 115.1: Steal Strength's second target is priced together
// with the first. Jubilant Skybonder gives every flyer its side controls
// "spells your opponents cast that target this creature cost {2} more", so a
// Spectral Adversary first target already costs {3}{B}, and Skybonder itself as
// the second target would make it {5}{B} against four available mana. Gorge
// withholds Skybonder from the second ask; dirac priced each candidate as the
// sole selection and offered it (tail-09-c000 pair 44, decision 223).
func TestMBCJubilantSkybonderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "raise_target_later_slot")
}
