package oracle

import "testing"

// Shiv's Embrace's "{R}: Enchanted creature gets +1/+0 until end of turn" is
// offered by the kernel at a priority decision where gorge offers only pass and
// another permanent's ability (decision 745 of 745; dr-mbc-abbb pair 17).
func TestMBCShivSEmbraceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "shivs_embrace_pump_offered_when_unaffordable")
}
