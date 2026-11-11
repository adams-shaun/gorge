package kgen

// extrablock: "can block an additional creature" (CanBlockAmount$ N) and
// "can block any number of creatures" (CanBlockAny$ True) on a Continuous
// static (CR 509.1a). Lowering one needs a blocker that blocks several
// attackers and the CR 510.1d split of that blocker's combat damage among
// them, chosen by its controller. dirac's Perm.blocking names one attacker,
// and gorge reads neither key (its only multi-attacker blockers are banding's
// and BlockAllDefined$'s), so there is no gorge behaviour to match: the
// static is refused by name instead of as an anonymous unread param.

// extraBlockRefusal refuses a static carrying either extra-block key.
func extraBlockRefusal(sp *params) error {
	for _, key := range []string{"CanBlockAmount", "CanBlockAny"} {
		if v, ok := sp.get(key); ok {
			return refuse("%s$ %q: a blocker of several attackers (CR 509.1a, 510.1d) is unmodelled in dirac and gorge", key, v)
		}
	}
	return nil
}
