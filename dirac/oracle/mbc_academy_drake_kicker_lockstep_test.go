package oracle

import "testing"

// Kicker {4} on Academy Drake: at priority for its controller, casting it
// kicked is a legal option whenever the kicker cost is affordable. The
// gorge side once offered only the non-kicked cast (decision 369 of 369;
// tail-66-c000 pair 121, seed 748975), while the kernel offered both.
func TestMBCAcademyDrakeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "academy_drake_kicker")
}
