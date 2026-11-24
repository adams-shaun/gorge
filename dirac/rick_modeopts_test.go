package dirac

import "testing"

// TestRickModeOptions: the as-enters mode ask offers one choice per mode, the
// index riding the action's mode (a Siege's two, Rick's three flag branches).
func TestRickModeOptions(t *testing.T) {
	for _, n := range []int{2, 3} {
		var d Decision
		siegeModeOptions(&d, n)
		if d.Sub != ChGenBranch || int(d.N) != n {
			t.Fatalf("n=%d: sub %v with %d options", n, d.Sub, d.N)
		}
		for m := 0; m < n; m++ {
			if got := d.Options[m].Mode(); int(got) != m {
				t.Errorf("n=%d: option %d has mode %d", n, m, got)
			}
		}
	}
}
