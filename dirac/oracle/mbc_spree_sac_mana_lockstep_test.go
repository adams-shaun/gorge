package oracle

import "testing"

// CR 601.2b, 702.172a, 605.1a: Lively Dirge (Spree) cast with the floating
// {U}{B} paying its printed {1}{B}, one untapped Swamp and Krark-Clan
// Ironworks (sacrifice an artifact: add {C}{C}, itself the artifact) beside.
// The second mode's {2} is payable only through the Ironworks' sacrifice, so
// gorge's PotentialMana offers it at the mode ask; dirac priced the mode from
// the tap-and-mana sources alone and offered mode 0 only (decision 463 of
// dr-mbc-bbbb pair 32, seed 419073).
func TestMBCLivelyDirgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "spree_mode_counts_sac_mana_source")
}
