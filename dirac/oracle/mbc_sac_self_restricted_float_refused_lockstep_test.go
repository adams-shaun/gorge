package oracle

import (
	"testing"
)

// CR 106.6 with CR 601.2g-h: an activation whose cost sacrifices its own
// source (Parcel Myr's "{2}, Sacrifice Parcel Myr: Draw a card") is still the
// Myr-ability activation it was announced as when its mana is spent, so the
// Myr Reservoir's restricted mana ("spend this mana only to cast Myr spells or
// activate Myr abilities") the CR 601.2g mana step floated pays it. The kernel
// once refused the floating mana at the payment: its payment descriptor read
// the source's post-sacrifice zone, found it no battlefield permanent, and
// tapped two Islands instead, leaving the Reservoir's mana floating
// (decision 383 of pool tail-39, projection Cards[].Flags and Pool).
func TestMBCGolgariSignetReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sac_self_restricted_float_refused")
}
