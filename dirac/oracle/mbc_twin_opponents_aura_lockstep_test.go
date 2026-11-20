package oracle

import "testing"

// Mire Boa's {G} Regenerate beside Flooded Grove's hybrid-fee filter and a
// Sea's Claimed Plains. The payment gorge's planner must find: tap the
// Sea's-Claimed land (an Island, it produces {U}), pay Grove's {G/U} fee with
// that {U} (CR 605.3a-b), and its {G}{G} pays the activation. gorge's prefix
// search once never walked the funding branch: scriptTwin's attachment scan
// read only the twin pair's controller's battlefield, so the opponent's Aura
// went unseen and the Island-Plains was cut as a twin of a plain Plains whose
// mana it does not make (the kernel's twinSlotOf scans the whole board and
// offered the play).
func TestMBCMireBoaReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "twin_opponents_aura")
}
