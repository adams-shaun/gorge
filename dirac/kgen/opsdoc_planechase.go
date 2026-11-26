package kgen

func init() {
	opsEntries = append(opsEntries,
		opsDoc{"ext:PlaneCurse", "OpExt ExtPlaneCurse (Obj the cursed player)", "Eloren Wilds' plane-source curse (DB$ Effect | StaticAbilities$ STCantCast beside Triggers$ PlaneswalkedTo self-exile, Duration$ Permanent): the targeted player may not cast spells until any player planeswalks. gorge registers the CantBeCast static (Caster$ Player.IsRemembered, RememberedPlayers the targeted player) with the UntilPlaneswalk lifetime -- effEffect's effectPlaneCurseTrigger consumes the Effect's own PlaneswalkedTo self-exile trigger as the registration's end condition, and rules' checkPlaneswalkTriggers ends every live registration at the next PlanarWalk (withoutUntilPlaneswalk); the consult is castRestrictionSources' registered arm, so the cast gate offers the cursed seat no spells. The kernel carries no effect token either: the bit lives on the cold planar record (planeSeat.curse, dirac/zone_planecurse.go), planeswalk clears it, castForbiddenCost reads it after the static scan. lower_planecurse.go refuses every other StaticAbilities$+Triggers$ shape by name. The lockstep probe (dirac/oracle/planechase_planebodies_lockstep_test.go); Eloren Wilds itself lowers with no refusal."},
	)
}
