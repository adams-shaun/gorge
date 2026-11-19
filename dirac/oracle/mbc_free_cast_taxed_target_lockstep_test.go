package oracle

import "testing"

// CR 601.2c, 601.2f, 601.2g, 118.9d: Urza, Lord High Artificer's exiled card is
// cast without paying its mana cost, but Kasmina, Enigmatic Mentor's {2} tax
// still applies to a spell that targets her controller's creature. The caster
// holds one untapped Plains and an Urza whose {T}-an-artifact mana ability has
// no artifact to tap: gorge's window census cannot price that source
// (castWindowSourceExcluded), so its target menu keeps the taxed creature
// (Yahenni, Undying Partisan for Repulse) however short the pool is. The
// kernel's free-cast menu skipped the window check and dropped it.
func TestMBCYahenniUndyingPartisanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_taxed_target_census_gap")
}
