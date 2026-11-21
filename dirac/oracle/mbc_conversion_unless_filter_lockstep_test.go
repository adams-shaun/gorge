package oracle

import "testing"

// CR 117.12 and CR 605.3b: Conversion's "sacrifice unless you pay {W}{W}"
// lets its controller activate Unknown Shores' {1},{T} any-colour mana
// ability beside a Plains (the {1} from an Island), so the payment is
// affordable and gorge offers both the pay and the decline mode. The kernel
// offered the decline mode alone (tail-34 pair 2, decision 457).
func TestMBCSeagrafSkaabReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "seagraf_skaab_conversion_unless_filter")
}
