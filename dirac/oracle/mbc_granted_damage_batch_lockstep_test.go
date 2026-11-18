package oracle

import "testing"

// CR 603.2c with CR 510.4: a Fungus Sliver (a Sliver granting every Sliver
// "whenever this creature is dealt damage") hit by two creatures in one combat
// damage step triggers once for the batch, however many creatures dealt the
// damage. Gorge latched a granted AddTrigger$ line by the address of its
// re-parsed line, which differs on each event, so the second hit queued a
// second trigger and gorge offered two trigger orders where dirac offered one
// (tail-24-c000 pair 7, decision 502).
func TestMBCFungusSliverReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_damage_done_once_one_trigger_per_batch")
}
