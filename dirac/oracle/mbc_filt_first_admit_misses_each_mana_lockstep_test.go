package oracle

import "testing"

// CR 605.3b, 605.1a: Cabal Coffers ("{2}, {T}: Add {B} for each Swamp you
// control") is a first-only filter (its only mana ability is the filter), so
// the script search admits it only once the other potential mana covers its
// {2}. Azorius Chancery's "{T}: Add {W}{U}" makes exactly those two, but the
// admission bound counted only the all-at-once source's split unit ({W}) and
// left its fixed rest ({U}) out, so with every Swamp tapped Coffers was never
// admitted and Korlash, Heir to Blackblade's "{1}{B}: Regenerate" was offered
// by gorge and not by the kernel (decision 491 of tail-41 pair 51).
func TestMBCKorlashHeirToBlackbladeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filt_first_admit_misses_each_mana")
}
