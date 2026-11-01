package oracle

import "testing"

// Downwind Ambusher's enters trigger with no opposing creature to target has no
// mode to choose, so it leaves the stack (CR 603.3c, 700.2a).
func TestModelessTriggerRemoved(t *testing.T) {
	synthPairs(t, "std-blb", []int{38, 106, 441}, nil)
}
