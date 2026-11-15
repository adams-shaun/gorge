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
	PrevScopeOppPlayer = 1 << 13
	PrevScopeAnyPlayer = 1 << 14
)
