package kgen

// cantActivate lowers a printed S:Mode$ CantBeActivated
// (gorge abilityRestrictionApplies, statics_cast.go): the activated abilities
// of battlefield permanents matching ValidCard$ (read with the static's
// controller as "you" and EnchantedBy as its source's host) can't be
// activated by the players Activator$ names (Opponent: the controller's
// opponents; You: the controller; absent: anyone), under Condition$
// (PlayerTurn: the controller's turn). ValidSA$ absent or Activated matches
// every activated ability, mana abilities included; Activated.!ManaAbility
// spares mana abilities (flag SpareMana, kop A = 1). The reach is the
// battlefield only: AffectedZone$ Battlefield, or no AffectedZone$ over a
// filter only an attached host matches (an Aura's EnchantedBy). Refused:
// any other ValidSA$ constraint (gorge denies fail-closed), Activator$,
// AffectedZone$, Phases$ and CheckSVar$ (unread).
func (lc *lowerCtx) cantActivate(sp *params, st *lStatic) error {
	vc, ok := sp.get("ValidCard")
	if !ok {
		return refuse("CantBeActivated without ValidCard$")
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	st.F = f
	// gorge restrictionGateHolds reads only a blank, PlayerTurn or
	// NotPlayerTurn Condition$ (any other withholds the restriction), and
	// checkSVarHolds the CheckSVar$ gate: staticBody's shared reader took
	// them, so only the verified PlayerTurn shape passes here
	if c, ok := sp.m["Condition"]; ok && c != "PlayerTurn" {
		return refuse("CantBeActivated Condition %q", c)
	}
	for _, k := range []string{"CheckSVar", "IsPresent"} {
		if _, ok := sp.m[k]; ok {
			return refuse("CantBeActivated %s$", k)
		}
	}
	if az, ok := sp.get("AffectedZone"); ok {
		if az != "Battlefield" {
			return refuse("CantBeActivated off the battlefield (AffectedZone %q)", az)
		}
	} else if !attachedOnly(f) {
		return refuse("CantBeActivated off the battlefield")
	}
	switch a := sp.str("Activator"); a {
	case "":
	case "You":
		st.Flags = append(st.Flags, "ActivatorYou")
	case "Opponent":
		st.Flags = append(st.Flags, "ActivatorOpp")
	default:
		return refuse("CantBeActivated Activator %q", a)
	}
	switch v := sp.str("ValidSA"); v {
	case "", "Activated":
	case "Activated.!ManaAbility":
		st.Flags = append(st.Flags, "SpareMana")
	default:
		return refuse("CantBeActivated ValidSA %q", v)
	}
	sp.consume("Secondary")
	return nil
}
