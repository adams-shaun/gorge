package oracle

import "testing"

// CR 106.4 with CR 601.2g: two Sages of the Maze ("{T}: Add two mana in any
// combination of colors") and two Islands pay Hidden Path's {2}{G}{G}{G}{G}
// (the Sages' four mana all green). Gorge's prefix search over the Combo Any
// sources skips a priority node whose pool and tapped set it has visited
// (scriptStateKey), so the (W,G) and (G,W) answers of the first Sage share one
// subtree and the all-green line is found inside its 160-node budget. The
// kernel's mirror visited every answer's subtree and ran out of nodes first, so
// it left the cast off the menu (decision 495, tail-36-c000 pair 61).
func TestMBCHiddenPathReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "comboany_two_sources_node_dedup")
}
