package oracle

import "testing"

// CR 400.7: Sawtooth Ogre blocks Glimmerpoint Stag (stolen by Abduction) and
// registers "deals 1 damage to that creature at end of combat". Both die in
// combat damage; Abduction's leaves trigger returns the Stag to the
// battlefield. The returned Stag is a new object, so the end-of-combat
// delayed damage finds no creature. Gorge once dealt the 1 damage to the
// returned Stag (decision 395 of tail-07 pair 91, seed 552030).
func TestMBCChamberSentryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delayed_damage_skips_returned_creature")
}
