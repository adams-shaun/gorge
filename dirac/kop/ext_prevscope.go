package kop

// The filtered form of ExtPreventDamageScope: a scoped prevention whose
// ValidSource$ or ValidTarget$ is a card filter (the Fog family).
const (
	// PrevScopeFiltered: the instruction's F is the damage source's filter
	// (0: any source) and B.N the filter the recipient permanents match (0:
	// none match). The record keeps only this bit; the kernel finds the
	// instruction again from the Effect's source card (dirac/
	// repl_prevscopefilter.go). A card carries one filtered pair.
	PrevScopeFiltered = 1 << 5
	// PrevScopeOppPlayer and PrevScopeAnyPlayer (instruction bits only):
	// the opponents of the resolving controller, or every player, are
	// prevented recipients (ValidTarget$ Opponent / Player); PrevScopePlayer
	// remains the controller alone.
	// PrevScopeGain (instruction bit only; the record keeps a rider byte,
	// genState.prevX): the record's controller gains life equal to the damage it
	// prevents (Chant of Vitu-Ghazi).
	// PrevScopeEndCombat (instruction bit only; the record keeps a rider byte,
	// genState.prevX, whose pxEOC bit marks it): the record ends with the combat
	// (Duration$ UntilEndOfCombat: Suppressor Skyguard; gorge
	// expireUntilEndOfCombat, CR 511.2/511.3). It never reaches the record's sc:
	// sc's bit 7 is prevNextSc (repl_prevnext.go).
	PrevScopeGain      = 1 << 6
	PrevScopeEndCombat = 1 << 7
	PrevScopeOppPlayer = 1 << 13
	PrevScopeAnyPlayer = 1 << 14
)
