package oracle

import (
	"testing"
)

// CR 114.3 with CR 611.3c: Arlinn Kord's transformed -6 makes an emblem whose
// static grants every creature its controller controls haste and a "{T}: This
// creature deals damage equal to its power to any target." activation. The
// gorge side once dropped the emblem's whole static on the floor -- its body
// carries AddSVar$ (the granted ability's ArlinnX), which the Effect-delivered
// static-grant readable check did not admit -- so the kernel's creatures kept
// the granted keyword while gorge's lost it (decision 558, projection
// Cards[i].Chars.Keywords; kfuzz synth tail-10 pair 80, the implicate Kraul
// Foragers beside Stuffed Bear and a Wolf token).
func TestMBCRibskiffReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "emblem_static_grant")
}
