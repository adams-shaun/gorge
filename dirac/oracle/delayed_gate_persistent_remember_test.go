package oracle

import "testing"

// Nezahal, Primal Tide's discard-three ability was activated twice in one
// turn. The first exiled it and registered its return-at-end-step delayed
// trigger; the second resolved with the card already in exile, remembered
// nothing itself, and its DelTrig sub's ConditionDefined$ Remembered gate
// still held on the host's persistent remembered list (gorge
// rememberedWithSource; Forge's SpellAbilityCondition.areMet reads
// getDefinedObjects(host, "Remembered")), so gorge registered a second
// delayed trigger. The kernel's lowered gate counted the resolution's own
// remembered set only and registered none, so the end step's triggers
// diverged: gorge asked the CR 603.3b trigger-order question over two
// pending triggers while the kernel pushed its single one, first difference
// projection NStack.
func TestDelayedGatePersistentRememberRepro(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "delayed_gate_reads_persistent_remember")
}
