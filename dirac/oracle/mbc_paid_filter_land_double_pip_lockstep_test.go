package oracle

import "testing"

// Coveted Falcon ({1}{U}{U}) with one Island and a Henge of Ramos ({2},{T}: any
// colour) among the lands: gorge offers the cast, paying the second {U} through
// the filter land; the kernel's menu left it out (tail-26-s000 pair 45,
// decision 670).
func TestMBCCovetedFalconReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "paid_filter_land_double_pip_cast")
}
