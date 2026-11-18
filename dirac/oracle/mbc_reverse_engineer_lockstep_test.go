package oracle

import "testing"

// CR 106.1, 605.1a, 601.2g: Reverse Engineer ({3}{U}{U}, improvise) is payable
// from two Islands, a Swamp wearing Sheltered Aerie ("{T}: Add two mana of any
// one color") and Skarrg, the Rage Pits ({C}), five mana with no artifact to
// improvise (tail-62 pair 72, decision 194 of 194).
func TestMBCReverseEngineerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reverse_engineer_aura_granted_double_mana")
}
