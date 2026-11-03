package oracle

import "testing"

// Downwind Ambusher's enters trigger with no opposing creature to target has no
// mode to choose, so it leaves the stack (CR 603.3c, 700.2a).
func TestModelessTriggerRemoved(t *testing.T) {
	synthPairsReaching(t, "std-blb", Config{GorgeAutopaySurface: true}, []int{15, 17, 5}, cardOut("Downwind Ambusher"))
}
