package cards

import "testing"

// TestSAChainMayAskExistingCounter: CounterType$ ExistingCounter picks a
// kind among the counters on the recipient, so the chain may ask and its
// resolution needs the kernel's checkpoint.
func TestSAChainMayAskExistingCounter(t *testing.T) {
	for _, src := range []string{
		"A:AB$ PutCounter | Cost$ 2 | ValidTgts$ Enchantment.YouCtrl | CounterType$ ExistingCounter | CounterNum$ 1 | SorcerySpeed$ True",
		"A:SP$ PutCounter | Defined$ Self | CounterType$ ExistingCounter",
	} {
		f := mayAskCard(t, "Name:Ask Probe\nManaCost:R\nTypes:Enchantment\n"+src+"\nOracle:x\n")
		sa := f.Abilities[0]
		if !SAChainMayAsk(sa, f.SVars, f, true) {
			t.Errorf("%s: SAChainMayAsk = false, want true", src)
		}
	}
}
