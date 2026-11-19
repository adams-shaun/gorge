package oracle

import "testing"

// CR 603.5: a granted trigger's may-election is posed at resolution (gorge
// askOptionalAtResolution: the line is the grant provenance recorded at push,
// never a face scan). Snake Umbra's Continuous grant put its "whenever
// enchanted creature deals combat damage to a player, you may draw a card" on
// Midnight Guard; when the granted trigger resolved, the kernel's
// cloneTrigMoved gate compared the entry's card (the granter's) against the
// host's live face, read the grant as a moved face and skipped the ask,
// drawing unasked while gorge asked (decision 369 of tail-06 focus game
// fx-one, pair 7).
func TestMBCGrantedOptMayAskFaceGateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_opt_may_ask_skipped_on_face_diff")
}
