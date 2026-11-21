package kgen

// charmRestrict reads a charm's ChoiceRestriction$ ("choose one that hasn't
// been chosen this turn / this game", gorge effects.CharmEligibleModes) into
// lc.ab.ModeRestrict (kernel modal_charmpick.go). Only a single-pick
// charm on a printed battlefield trigger or activated ability reads it, one
// per card (the kernel's pick log is keyed by the source alone); YourLastCombat
// and every other shape refuse.
//
// ThisTurn and ThisGame lower on a battlefield trigger: gorge asks a trigger
// charm's modes when the trigger is put on the stack and records the pick on
// the source then (rules/resolution_modes.go drainAwaitsModes ->
// effects.RecordCharmChoices), so a countered trigger still logged its mode
// and the next instance sees it; the kernel's pick log (cpGame for ThisGame,
// ModeRestrict 2) records at the same moment (Demonic Pact). An activated
// charm (Three Bowls of Porridge, ThisGame) lowers the same way: gorge asks
// its modes at activation (ResumeKind "cast_modes") with ResumeModes set to
// the eligible modes, so the answer maps against the filtered list and the
// pick log is recorded then; the kernel's activated half (modal_charmpick.go,
// modal_activated.go) matches that offer.
func (lc *lowerCtx) charmRestrict(ps *params) error {
	v, ok := ps.get("ChoiceRestriction")
	if !ok {
		return nil
	}
	switch v {
	case "ThisTurn":
		lc.ab.ModeRestrict = 1
	case "ThisGame":
		lc.ab.ModeRestrict = 2
	default:
		return refuse("charm ChoiceRestriction$ %q", v)
	}
	if lc.ab.ModesPick != 1 || lc.ab.ModesMin != 1 {
		return refuse("charm ChoiceRestriction$ with CharmNum$ %d / MinCharmNum$ %d", lc.ab.ModesPick, lc.ab.ModesMin)
	}
	switch lc.ab.Kind {
	case akTriggered:
		if tr := lc.ab.Trigger; tr == nil || tr.Zone != 0 && tr.Zone != zBattlefield {
			return refuse("charm ChoiceRestriction$ on a trigger outside the battlefield")
		}
	case akActivated:
	default:
		return refuse("charm ChoiceRestriction$ on a %s ability", abilityKindNames[lc.ab.Kind])
	}
	for _, o := range lc.out.Abilities {
		if o != lc.ab && o.ModeRestrict != 0 {
			return refuse("two ChoiceRestriction$ charms on one card")
		}
	}
	return nil
}
