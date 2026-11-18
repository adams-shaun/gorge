package oracle

import "testing"

// Dr. Madison Li ({U}{R}{W}) is offered as a cast by the kernel at decision 895
// of tail-07-c000 pair 54 while gorge omits it.
func TestMBCDrMadisonLiReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dr_madison_li_cast_offered")
}
