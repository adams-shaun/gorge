package oracle

import "testing"

// Righteous Confluence was castable for gorge but missing from the kernel's
// priority options (tail-45-c000, decision 212 of pair 21; seed 505316).
func TestMBCRighteousConfluenceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "righteous_confluence_cast_offer")
}
