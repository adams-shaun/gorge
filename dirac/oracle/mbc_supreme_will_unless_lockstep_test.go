package oracle

import "testing"

// Supreme Will's counter (mode 0) on p0's own Blood Curdle resolves into the
// unless ask "counter it unless you pay {3}": the {3} is payable (an unless
// payment is no spell's spend, CR 106.6), so gorge offers both answers, pay
// and decline. The kernel priced the {3} unpayable and offered decline only:
// decision 984 of the recorded game, dr-mbc-aaab pair 187 (gorge leads,
// options options: modes/mode of the unless ask).
func TestMBCSupremeWillReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "supreme_will_unless_payable")
}
