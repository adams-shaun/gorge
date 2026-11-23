package oracle

import "testing"

// CR 702.26b with CR 701.54a: a phased-out creature is treated as though it
// doesn't exist, so "the Ring tempts you" cannot make it the Ring-bearer. Haystack
// phased Frost Lynx out, then Horses of the Bruinen's temptation offered gorge
// four creatures where the kernel offered three (decision 935 of tail-29-c000
// pair 35). Frost Lynx must not be among the choices.
func TestMBCFrostLynxReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ring_bearer_choice_skips_phased_out")
}
