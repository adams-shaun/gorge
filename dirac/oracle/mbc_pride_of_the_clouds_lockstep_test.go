package oracle

import "testing"

// Pride of the Clouds is targetable by Kor Line-Slinger ("power 3 or less")
// only while its +1/+1 per other flier leaves its power at 3 or less; the two
// engines disagreed on that offer at decision 919 (tail-20-c000, pair 1).
func TestMBCPrideOfTheCloudsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "pride_of_the_clouds_power_target")
}
