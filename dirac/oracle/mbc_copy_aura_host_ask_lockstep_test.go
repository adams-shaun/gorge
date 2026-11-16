package oracle

import "testing"

// CR 603.7: the two token copies one Venser, Fervent Forger mode resolution
// makes share one "sacrifice them at the beginning of the next end step"
// delayed triggered ability. When the copied permanent is an Aura (Custody
// Battle) each copy asks its host as it enters, and the kernel registered a
// sacrifice trigger per copy from the second ask on, offering a second
// trigger_order option for the same source that gorge never has.
func TestMBCVenserFerventForgerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "copy_aura_host_ask_splits_sacrifice_group")
}
