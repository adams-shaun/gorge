package oracle

import "testing"

// CR 122.1 with Dramatist's Puppet's "for each kind of counter on target
// permanent, put another counter of that kind on it or remove one from it":
// the Puppet's trigger targets a Watchful Blisterzoa holding an oil and a
// +1/+1 counter. gorge elects over its counter slice's insertion order (oil
// first), the kernel walks kinds ascending (+1/+1 first), so the first election
// was posed about a different kind on each side (gorge 16 = oil, dirac 0).
// The kernel now stalls on the ask (StallCtrOrder) instead of diverging.
func TestMBCWatchfulBlisterzoaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "add_or_remove_each_existing_kind")
}
