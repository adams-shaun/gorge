package kgen

// lower_zonesearchcounters.go lowers a library search's
// WithCountersType$/WithCountersAmount$ rider (Forge api:ChangeZone with
// Origin$ Library, Destination$ Battlefield: Captain Jean-Luc Picard's
// "search your library for a Spacecraft card, put it onto the battlefield
// with seven charge counters"). gorge folds a battlefield entry's rider into
// the MoveZone event itself (effects.entryWithCounters,
// engine/effects/zone_counter_types.go) so the found permanent enters with
// the counters under CR 122.6; the kernel's OpExt SearchCounters arms the
// same entry around OpSearchEx (dirac/ctr_search.go).
//
// counterDestination (engine/effects/zone_hand.go) is battlefield or exile.
// A search to hand, library, graveyard or stack drops the rider exactly as
// gorge does. A search to exile is refused: gorge moves the card and then
// emits the counters as a CounterChange (effects/zone_search.go), a shape
// the kernel's single entry arm cannot carry, and one that was refused
// before any search rider lowered at all.
func (lc *lowerCtx) searchCounterArm(dest int, counterKind string, counters, n lValue, each bool) (arm, clear *lInstr, err error) {
	if counterKind == "" {
		return nil, nil, nil
	}
	switch dest {
	case zBattlefield:
	case zExile:
		return nil, nil, refuse("library search to exile with WithCountersType$")
	default:
		return nil, nil, nil
	}
	kind, ok := counterKinds[counterKind]
	if !ok {
		return nil, nil, refuse("WithCountersType$ %q", counterKind)
	}
	if !counters.isConst() || counters.N < 1 || counters.N > 255 {
		return nil, nil, refuse("WithCountersAmount$ %v on a library search", counters)
	}
	if !isConstN(n, 1) {
		return nil, nil, refuse("search onto the battlefield for %v cards with counters", n)
	}
	if each {
		// The arm covers one entrant; an EACH pick would leave the later
		// cards bare.
		return nil, nil, refuse("EACH library search onto the battlefield with counters")
	}
	return &lInstr{Op: "SearchCounters", B: counters, C: int(kind)},
		&lInstr{Op: "SearchCounters", B: cst(0)}, nil
}
