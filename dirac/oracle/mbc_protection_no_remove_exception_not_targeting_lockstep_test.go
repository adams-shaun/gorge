package oracle

import "testing"

// CR 702.16b with CR 702.16c: p1's Tattoo Ward sits on p0's Flitterstep
// Eidolon (an enchantment creature), so the Eidolon has protection from
// enchantments. The ward's sacrifice-destroy activation is an ability from an
// enchantment source, so the Eidolon is not a legal target for it: the ward's
// "This effect doesn't remove Tattoo Ward" exception spares only the removal
// the CR grants protection against, never targeting. Gorge once offered the
// Eidolon on this menu; dirac withheld it
// (tail-04-c000 pair 0, decision 285, lead gorge).
func TestMBCFlitterstepEidolonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "protection_no_remove_exception_not_targeting")
}
