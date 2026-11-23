package oracle

import "testing"

// CR 603.3d with CR 707.2: an embalmed Vizier of Many Faces entered as a copy
// of Disciple of Perdition and died to Cobblebrute's combat damage. Its copied
// "When this creature dies, choose one" trigger announced the exile mode at
// placement, and the chosen mode's target ("exile target opponent's
// graveyard") must be chosen at that same placement moment, before priority.
// The mode bodies resolved against the source's active face -- Vizier's, whose
// SVars lack the mode bodies -- so the target ask was never posed and the
// ability resolved untargeted; only one engine offered the target decision
// (dr-tail-49-c001 pair 1, seed 685921, before MBC was split).
func TestMBCCobblebruteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copied_trigger_mode_targets")
}
