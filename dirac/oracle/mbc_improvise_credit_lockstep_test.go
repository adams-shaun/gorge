package oracle

import "testing"

// CR 702.126a with CR 108.4a and CR 601.2f: p1's Archway of Innovation emblem
// ("the next spell you cast this turn has improvise") reads p1's exiled
// Slagwurm Armor as an improvise spell, but p0's Dragon's Smile cast of it taps
// p0's own untapped artifact (Zephyr Boots) for the {1}. gorge's offer base
// credits the taps of the seat offered the cast; the kernel read the keyword
// off that seat too, saw no improvise, priced the cast at {1} against an empty
// pool and withheld it (decision 972 of tail-15 pair 5, autopay surface).
func TestMBCSlagwurmArmorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "improvise_emblem_credit_reads_owner_taps_caster")
}
