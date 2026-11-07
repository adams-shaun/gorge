package kgen

import "strconv"

// reduceAbility lowers a ReduceCost static of activated abilities (Type$
// Ability, or no Type$ with ValidSpell$ Activated.Exhaust,
// Activated.PowerUp or Activated.Equip) and of Room unlock special actions (ValidSpell$
// Static.Unlock): "activated abilities of <permanents you control> cost {N}
// less", "unlock costs you pay cost {N} less" (Glimmer), and "power-up
// abilities of <permanents you control> cost {N} less" (Hulk, Gamma Goliath)
// (gorge statics_costapply.go costStaticGateFull). It reads exactly the shape
// kop.StatReduceAbility prices: the battlefield, the controller's own
// activations, ValidCard$ matched against the ability's source (or the
// unlocked Room), an Exhaust-, Power-up- or Equip-only restriction (the
// equip arm optionally gated on its target, ValidTarget$), and the unlock
// arm's B bit. Any other Activated.<class> and anything else refuses with a
// named reason.
func (lc *lowerCtx) reduceAbility(sp *params, st *lStatic, readF func(string) (*lFilter, error)) error {
	arm := 1
	if vs, ok := sp.get("ValidSpell"); ok {
		switch vs {
		case "Activated.Exhaust":
			arm = 2
		case "Static.Unlock":
			arm = 3
		case "Activated.PowerUp":
			arm = 4
		case "Activated.Equip":
			arm = 5
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
		// an equip's source is always a battlefield permanent: the equip
		// arm reads an absent AffectedZone$ as the battlefield
		if az := sp.str("AffectedZone"); az != "Battlefield" && (arm != 5 || az != "") {
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
		if _, ok := sp.get(k); ok && (arm != 5 || k != "ValidTarget") {
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
	if arm == 5 {
		// "Equip abilities you activate that target <X>" (Cloud, Planet's
		// Champion; Dwarven Mauler): no ValidCard$ is any permanent's equip,
		// and ValidTarget$ gates on the equip's target (kop.StTargetsFilter2,
		// read with the static's source as Self). A Spell head refuses:
		// an equip targets a creature, never a spell.
		if st.F == nil {
			if st.F, err = lc.filter("Card"); err != nil {
				return err
			}
		}
		if st.F2, err = readF("ValidTarget"); err != nil {
			return err
		}
	}
	if st.Amount, err = lc.value(sp.str("Amount")); err != nil {
		return err
	}
	return nil
}
