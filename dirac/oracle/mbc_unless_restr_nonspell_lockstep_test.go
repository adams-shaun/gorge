package oracle

import "testing"

// CR 106.6 with CR 117.5: an unless tax ("sacrifice this artifact unless you
// pay {1}", Kataki, War's Wage's granted upkeep trigger) is no spell cast, so
// the payer's floating mana whose spend restriction bars only casting
// ("this mana can't be spent to cast nonartifact spells", Karn, Legacy
// Reforged's persistent {C}) pays it, and the pay/decline election offers the
// pay. The kernel priced the {1} from the plain pool and the tap window only,
// never from the admitted floating batch, and offered the decline alone
// (decision 457 of 457; dr-mbc-abab pair 56).
func TestMBCUnlessRestrNonspellReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "unless_restr_nonspell")
}
