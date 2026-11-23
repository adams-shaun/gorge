package oracle

import "testing"

// CR 601.2g-h, 605.1a: Deep-Sea Serpent ({4}{U}{U}) is castable with three
// creatures under Cryptolith Rite ("{T}: Add one mana of any color"), a
// Forest, an Island and Wooded Bastion's "{T}: Add {C}". Gorge's legal walk
// prices the cast from its availability census, which counts the grants, and
// offers it; the kernel's plain offer gate does too, but grantCap then re-ran
// its 160-node grant search over the four gap sources. That search counted
// Viridian Acolyte's first-and-only filter fee as an extra funding step on an
// unrelaxable census that does not carry it, so its bound was one too high
// and it ran out of budget. The fixed funding count matches the census and
// finds the cast's script (decision 159, tail-15 pair 24 of the evidence
// loop's focus deck).
func TestMBCDeepSeaSerpentReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "grant_search_budget_plain_cast")
}
