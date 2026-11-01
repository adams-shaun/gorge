package kgen

// raiseCost lowers S:Mode$ RaiseCost for spells (gorge costModifiersCompose's
// raise group, CR 601.2f: raises before reductions, generic never below
// zero): Type$ Spell, ValidCard$ the spells, a constant or SVar Amount$ of
// generic mana, Activator$ You / Opponent / Player (absent: any caster),
// the static's conditions; EffectZone$ All with ValidCard$ Card.Self is the
// card's own "costs more". Refused: Cost$ (a non-generic raise), Relative$,
// ValidSpell$/AffectedZone$, abilities (Type$ Ability).
func (lc *lowerCtx) raiseCost(sp *params, st *lStatic) error {
	if sp.str("Type") != "Spell" {
		return refuse("RaiseCost Type %q", sp.str("Type"))
	}
	var err error
	if vc := sp.str("ValidCard"); vc != "" {
		if st.F, err = lc.filter(vc); err != nil {
			return err
		}
	}
	if st.Amount, err = lc.value(sp.str("Amount")); err != nil {
		return err
	}
	if vt, ok := sp.get("ValidTarget"); ok {
		// a raise that applies once a chosen target matches (Dragon's Prey;
		// gorge costTargetsMatch): read as the ReduceCost arm reads it
		lc.spellBaseOK = true
		st.F2, err = lc.filter(vt)
		lc.spellBaseOK = false
		if err != nil {
			return err
		}
	}
	switch a := sp.str("Activator"); a {
	case "", "Player":
	case "You":
		st.Flags = append(st.Flags, "ActivatorYou")
	case "Opponent", "Player.Opponent":
		st.Flags = append(st.Flags, "ActivatorOpp")
	default:
		return refuse("RaiseCost Activator %q", a)
	}
	if ez, ok := sp.get("EffectZone"); ok {
		if ez != "All" || sp.str("ValidCard") != "Card.Self" {
			return refuse("RaiseCost EffectZone %q", ez)
		}
		st.Zone = zAny
	}
	return nil
}
