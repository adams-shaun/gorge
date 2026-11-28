package oracle

import "testing"

const thunderscapeMasterRepro = "thunderscape_master_gap_search_budget_exhausted"

// CR 601.2g: Thunderscape Master's cast costs {2}{R}{R}. With two Mountains, a
// Domesticated Watercourse, five Islands under Scald and Overflowing Basin's
// {1},{T} filter untapped, gorge's potential-play prefix search for that cast
// walks its whole potentialScriptNodes budget (plain, phased and subsume runs)
// and prices nothing, so the autopay surface withholds the cast; the kernel's
// mirror of the search prices the payment and offers it (tail-49 pair 60,
// decision 608, "options options"). That was the divergence the card was
// refused for. At the recorded decision both engines now offer the cast of
// Thunderscape Master, and the recorded game replays clean on a table of its
// own cards, so the card is unrefused with this game as its proof
// (dirac/support/unrefused.tsv).
func TestMBCThunderscapeMasterReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, thunderscapeMasterRepro)
}
