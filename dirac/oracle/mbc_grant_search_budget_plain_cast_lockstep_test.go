package oracle

import "testing"

// CR 601.2g-h, 605.1a: Deep-Sea Serpent ({4}{U}{U}) is castable with three
// creatures under Cryptolith Rite ("{T}: Add one mana of any color"), a
// Forest, an Island and Wooded Bastion's "{T}: Add {C}". Gorge's legal walk
// prices the cast from its availability census, which counts the grants, and
// offers it; the kernel's plain offer gate does too, but grantCap then re-ran
// its 160-node grant search over the four gap sources and removed the cast
// when that search ran out of budget. The legal census already priced it, so
// a budget-cut search must not withdraw the offer (decision 159, tail-15
// pair 24 of the evidence loop's focus deck).
func TestMBCDeepSeaSerpentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_budget_plain_cast")
}
