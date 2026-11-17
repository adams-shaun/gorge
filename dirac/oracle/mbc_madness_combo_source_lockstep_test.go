package oracle

import "testing"

// CR 601.2g with CR 702.35a: Krosan Archer's "{G}, Discard a card" discards
// Insatiable Gorgers (madness {3}{R}) to its controller's board of two
// Mountains and a Ballroom ("{T}: Add {W} or {B}") with the rest tapped. The
// madness ask must not offer the cast: the Ballroom makes one mana, so the
// three untapped lands pay at most three of the four. Gorge priced the ask
// against PotentialMana's per-colour bound, which counts a Combo source in
// both its colours, and offered a yes it could not pay (first difference
// decision 417 of tail-61 pair 91, options only gorge offering yes(Gorgers)).
func TestMBCInsatiableGorgersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_counts_a_combo_source_once")
}
