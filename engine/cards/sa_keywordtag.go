package cards

// IsCrewAbility reports whether sa is the activated ability a Crew keyword
// expands to (CR 702.122): engine/cards/kw_crew.go's crewAbilitySA mints it
// with exactly `Keyword$ Crew`, for the printed expansion and the granted
// route alike. It is the resolution-side twin of pay.SaHasKeyword(sa, "Crew"),
// which engine/effects cannot import.
func (sa *SA) IsCrewAbility() bool {
	return sa != nil && sa.ParamStr(PKKeyword) == "Crew"
}
