package dirac

// An activation whose own {T} is part of its cost (Immersturm Skullcairn's
// "{1}{B}{R}{R}, {T}, Sacrifice: ..." beside an Incubation Druid) is priced by
// gorge's potential walk against PotentialMana, which counts the source's own
// mana: one tap cannot pay both, but the walk is only the bound that lists the
// play as a candidate. The planner's census then finds it insufficient (the
// source's own tap is no unit of its payment) and the prefix search taps the
// census gap (the Druid, reflecting the Skullcairn's {B}) to pay it for real
// (CR 602.2b, 106.7). The gaps walk (availGaps) removes the source's own mana
// (abilityAffords), so a play only that mana made affordable fell out of the
// candidates. availOwn keeps it in.

// ownTapWalkPlay reports whether activation a is a candidate of gorge's
// potential walk that the gaps walk dropped for its own {T} source's mana. The
// walk is built once per scriptOffers call (built, walk).
func (s *State) ownTapWalkPlay(k *prep, p uint8, a Action, walk *Decision, built *bool) bool {
	if a.Kind() != AkActivate {
		return false
	}
	if !*built {
		*built = true
		s.plainOptions(k, walk, availGaps|availOwn)
	}
	return hasOption(walk, a)
}

// eachOnlyOwnTapPlay reports whether play a is a candidate only gorge's
// PotentialMana bound lists (ownTapWalkPlay: the activation's own {T} source
// counted in the census) on a board whose only gaps are free each-colour
// sources (Faeburrow Elder). The gaps walk does not list it, so the planner
// finds the play insufficient and gorge's prefix search taps the gaps for
// real: it offers the play only when that search finds a script, as the
// restricted and counter-cost gaps' search does (scriptDeadEnd; CR 602.2b).
// unrelaxed: another arm (twoLive, repeatLive) already owns the play's search.
func (s *State) eachOnlyOwnTapPlay(k *prep, p uint8, a Action, gaps *Decision, unrelaxed bool) bool {
	return !unrelaxed && a.Kind() == AkActivate && !hasOption(gaps, a) && s.scriptGaps(k, p).any()
}
