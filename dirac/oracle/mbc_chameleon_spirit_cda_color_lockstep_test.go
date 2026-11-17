package oracle

import "testing"

// CR 604.3 and CR 613.1e: Chameleon Spirit's characteristic-defining P/T counts
// the permanents of its chosen colour its controller's opponents control, and
// that count reads the layer-5 colours continuous effects give those permanents
// (Cloudchaser Kestrel's "target permanent becomes white until end of turn").
// The Island was colourless on its printed face; the Kestrel made it white, and
// gorge's count missed that layer-5 colour while the kernel counted it
// (Cards[45].Power 1 vs 2, CR 613.1e before 613.4a).
func TestMBCChameleonSpiritReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "chameleon_spirit_cda_color")
}
