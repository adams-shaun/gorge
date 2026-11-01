package kgen

// ReplaceDyingDefined$ Targeted on a Pump or Fight
// ("if that creature would die this turn, exile it instead"; CR 614.1a,
// 614.9). gorge effects/damage.go registerReplaceDying registers one
// until-end-of-turn Moved replacement over the objects the pump affected
// (effects/pump.go pumpedTargets) or, for a Fight, the fighters narrowed to
// the chosen target (effFight): Battlefield to Graveyard becomes exile. The
// kernel's per-slot exile-instead mark is that replacement; the mark lands
// whether or not the creature took damage.

// replaceDyingMark emits the ExileExMark of a Pump or Fight whose
// ReplaceDyingDefined$ is exactly Targeted over the effect's own target slot
// obj. Absent the param it emits nothing; any other spelling or subject
// refuses by name. A Fight passes its fighter (Defined$): gorge fightPairs
// drops the pair when either member is off the battlefield or not a
// creature, and then registers nothing, so the kernel op marks only while
// both are (Obj2; a Pump passes the zero ref).
func (lc *lowerCtx) replaceDyingMark(ps *params, obj, fighter lRef) error {
	rd, ok := ps.get("ReplaceDyingDefined")
	if !ok {
		return nil
	}
	if rd != "Targeted" || obj.Kind != rTarget {
		return refuse("ReplaceDyingDefined$ %q on a Pump/Fight", rd)
	}
	if fighter.Kind == rDelayedRemembered {
		return refuse("ReplaceDyingDefined$ on a Fight of a reflexive capture")
	}
	lc.emit(lInstr{Op: "ExileExMark", Obj: obj, Obj2: fighter})
	return nil
}
