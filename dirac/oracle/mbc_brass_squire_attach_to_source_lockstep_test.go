package oracle

import "testing"

// CR 301.5c and 701.3a: Brass Squire's "{T}: Attach target Equipment you
// control to target creature you control" attaches the CHOSEN Equipment (its
// ParentTarget), not the Squire itself, so the Squire is a legal creature
// target of its own ability. Only an Attach that fastens its own source (an
// Equip/Reconfigure expansion, Object$ absent or Self) excludes that source
// from the destination pool.
func TestMBCBrassSquireReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "brass_squire_attach_to_source")
}
