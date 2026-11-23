package oracle

import "testing"

// CR 106.7 with CR 605.3a: an Incubation Druid that has its +1/+1 counters
// taps for three mana of the one colour its ask names (an Island's {U} here),
// and a Boros Signet beside it makes the payment of a later {U} ability
// (Soliton's untap) go through the filter search, which took the Druid as a
// Combo Any step: the answer split the three mana across colours ({W}{W}{U})
// where gorge floats {U}{U}{U}. The pools part at decision 568.
func TestMBCReflAmtColourOrderReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"reflamt_filter_chain_amount"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			stdReproNamed(t, name)
		})
	}
}
