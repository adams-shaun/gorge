package oracle

import "testing"

// CR 616.1: Avenging Druid's combat damage to the defending player is matched
// by two replacements at once: Commencement of Festivities's "prevent all
// combat damage that would be dealt to players this turn" (a full prevention)
// and the opposing Battletide Alchemist's optional "you may prevent X of that
// damage". Whichever is applied first, the full prevention leaves nothing
// dealt, but gorge asks the affected player the order, with the optional
// line's skip. dirac's combat walk stalled (replplus) on the optional line
// beside a competing full prevention instead of posing that ask (decision 752
// of pool tail-76 pair 56).
func TestMBCAvengingDruidReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "optional_prevention_beside_full_prevention")
}
