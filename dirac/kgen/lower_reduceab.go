package kgen

import "strconv"

// reduceAbility lowers a ReduceCost static of activated abilities (Type$
// Ability, or no Type$ with ValidSpell$ Activated.Exhaust) and of Room unlock
// special actions (ValidSpell$ Static.Unlock): "activated
// abilities of <permanents you control> cost {N} less" and "unlock costs you
// pay cost {N} less" (Glimmer) (gorge
// statics_costapply.go costStaticGateFull). It reads exactly the shape
// kop.StatReduceAbility prices: the battlefield, the controller's own
// activations, ValidCard$ matched against the ability's source (or the
// unlocked Room), an Exhaust-only restriction, and the unlock arm's B bit.
// Anything else refuses with a named reason.
func (lc *lowerCtx) reduceAbility(sp *params, st *lStatic, readF func(string) (*lFilter, error)) error {
	arm := 1
	if vs, ok := sp.get("ValidSpell"); ok {
		switch vs {
		case "Activated.Exhaust":
			arm = 2
		case "Static.Unlock":
			arm = 3
		default:
			return refuse("ReduceCost ValidSpell %q", vs)
		}
	}
	if t, ok := sp.get("Type"); ok && t != "Ability" {
		return refuse("ReduceCost Type %q", t)
	}
	if arm == 3 {
		if sp.str("Type") == "Ability" {
			return refuse("ReduceCost Type$ Ability with ValidSpell$ Static.Unlock")
		}
		// AffectedZone$ is optional for the unlock arm: the special action's
		// subject is always a battlefield Room. A present value must say so.
		if az := sp.str("AffectedZone"); az != "" && az != "Battlefield" {
			return refuse("ReduceCost AffectedZone %q", az)
		}
		if _, ok := sp.get("MinMana"); ok {
			return refuse("ReduceCost MinMana$ with ValidSpell$ Static.Unlock")
		}
	} else {
		if az := sp.str("AffectedZone"); az != "Battlefield" {
			return refuse("ReduceCost AffectedZone %q", az)
		}
		if v, ok := sp.get("MinMana"); ok {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 15 {
				return refuse("ReduceCost MinMana$ %q", v)
			}
			st.MinMana = uint8(n)
		}
	}
	st.AbilityRed = uint8(arm)
	for _, k := range []string{"Color", "ValidTarget", "EffectZone"} {
		if _, ok := sp.get(k); ok {
			return refuse("ReduceCost %s$", k)
		}
	}
	if a, ok := sp.get("Activator"); ok && a != "You" {
		return refuse("ReduceCost Activator %q", a)
	}
	var err error
	if st.F, err = readF("ValidCard"); err != nil {
		return err
	}
	if st.Amount, err = lc.value(sp.str("Amount")); err != nil {
		return err
	}
	return nil
}
