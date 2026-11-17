package dirac

// A Taps-deferred gap (a depletion land, Hickory Woodlot: "{T}, Remove a
// depletion counter: Add {G}{G}", a bounded rider gorge's planner defers,
// mana_rider.go; a Taps watcher's deferred source, mana_tapsdefer.go) beside a
// filter (Viridescent Bog's "{1},{T}: Add {B}{G}"): gorge's census lists
// neither, and its one prefix search (rules scriptSearch.visit) activates every
// census gap in battlefield order, so the filter search takes the deferred
// source as an otherGap step, tapped as the script search taps it
// (scriptGapTap): the Woodlot's {G}{G}, one {G} paying the Bog's {1}, and the
// Bog's {B}{G} pay Retrofitter Foundry's "{3}: Untap" (CR 605.3a, 601.2g-h).
// The tap-another search's twin is mana_tapxdefer.go, the grant search's
// mana_granttapsdefer.go.

// The gap step itself is otherGap's Taps-deferred arm (mana_mixgaps.go,
// tapsDeferGap); this file keeps the potential-mana count a first-only
// filter's fee draws on.

// filtRiderMana is the mana p's Taps-deferred rider gaps (Hickory Woodlot's
// {G}{G}) add to the potential a first-only filter's fee draws on: gorge's
// PotentialMana counts a bounded rider source (mana_rider.go), so a Woodlot
// alone funds Viridescent Bog's {1}. The plain walk's canTapForMana skips the
// rider (its cost removes a counter), so filtOtherMana counts it here.
func (s *State) filtRiderMana(k *prep, p uint8) int {
	var av avail
	for b := s.scriptTapsDefer(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if _, ok := s.riderAbilityOf(k, sl); ok {
			s.addTapsDeferUnits(k, sl, &av)
		}
	}
	return av.total
}
