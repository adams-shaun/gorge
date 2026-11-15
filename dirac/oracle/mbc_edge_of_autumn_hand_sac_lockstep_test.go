package oracle

import "testing"

// CR 702.29a with 118.3: Edge of Autumn's cycling costs "Sacrifice a land,
// Discard this card", so the hand activation is a priority option only when
// the payer controls a land to sacrifice. At the game's first priority (no
// land has entered the battlefield, the stack is empty) the kernel offered
// activate(Edge of Autumn) beside pass while gorge priced the sacrifice part
// unpayable and offered pass alone (synth tail-06 pair 1, seed 15932193312723074476).
func TestMBCEdgeOfAutumnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_edge_of_autumn_hand_sac")
}
