package oracle

import "testing"

// The recorded game: at decision 328 dirac offers casts of A-Cauldron
// Familiar, Circle of Dreams Druid and Hand of Death that gorge's priority
// decision leaves out (tail-55 pair 55).
func TestMBCACauldronFamiliarReprosLockstep(t *testing.T) {
	stdReproNamed(t, "cauldron_familiar_options")
}
