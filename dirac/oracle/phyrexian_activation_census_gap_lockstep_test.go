package oracle

import "testing"

// CR 107.4f, CR 602.2b: Lashwrithe's equip costs {B/P}{B/P}. At life 2 with an
// empty pool, one pip is paid with two life and the other needs {B}. The only
// such source is Hall of Tagsin's "{1}, {T}: Add one mana of any color", a mana
// ability with its own mana cost — a census gap in gorge's planner census. The
// kernel's Phyrexian activation gate once recomputed its affordability over a
// census with that source excluded, and the lists parted at decision 943 of
// this recorded tail-39 game (gorge withholding). gorge's potential witness
// (cr-phyrexian-activation-low-life) prices the mana half with its planner,
// which reaches the gap source through the script route, so gorge makes the
// play; the recompute is gone and both engines offer the activation, the
// recording replays clean.
func TestPhyrexianActivationCensusGapReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_activation_census_gap")
}
