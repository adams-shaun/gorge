package kgen

import (
	"slices"
	"strings"
)

// isOriginLibraryGrave reports a ChangeZone over Origin$ Library,Graveyard
// ("search your library and graveyard for ..."). gorge effChangeZone sends a
// mixed hidden origin with no object selector to effSearchLibrary: the pool is
// the fetcher's library, then their graveyard (zone_search.go's candidate
// walk), one pick, and the library shuffle tail. With no OriginAlternative$
// there is no zone election (originAlternativeZones returns the zones as
// parsed), so it lowers as the OriginAlternative$ Graveyard search with the
// election settled as both zones (kop.SrAltAll).
//
// Graveyard,Library (Doomsday, Chandra, Heart of Fire) is the same search:
// gorge's candidate walk puts the library window first whatever the
// Origin$ order (zone_search.go: the library, then every other origin zone).
func isOriginLibraryGrave(ps *params) bool {
	if o := strings.TrimSpace(ps.m["Origin"]); o != "Library,Graveyard" && o != "Graveyard,Library" {
		return false
	}
	_, alt := ps.m["OriginAlternative"]
	return !alt // with an alternative the election is real; zoneOf refuses the pair by name
}

// libGraveFlags is the Library,Graveyard search's guard and flag: a hand or
// battlefield destination, one card, a search that states a quality, the
// controller's own library, no entry counters. Shuffle$ False keeps its
// NoShuffle flag (the kernel's shuffle tail reads it whatever the zones).
func libGraveFlags(ps *params, dest int, n lValue, obj lRef, arm bool, flags []string) ([]string, error) {
	if dest == zLibrary {
		return libGraveToLibrary(n, obj, arm, flags)
	}
	switch {
	case dest != zHand && dest != zBattlefield:
		return nil, refuse("Origin$ Library,Graveyard search to %s", zoneLabel[dest])
	case slices.Contains(flags, "Mandatory"):
		return nil, refuse("Origin$ Library,Graveyard search with no stated quality or Mandatory$")
	case slices.Contains(flags, "DifferentNames"):
		return nil, refuse("Origin$ Library,Graveyard search with DifferentNames$")
	case obj.Kind != rYou:
		return nil, refuse("Origin$ Library,Graveyard search with DefinedPlayer$")
	case arm:
		return nil, refuse("Origin$ Library,Graveyard search with WithCountersType$")
	case !isConstN(n, 1):
		return nil, refuse("Origin$ Library,Graveyard search with ChangeNum$ %v", n)
	}
	return append(flags, "alt=Graveyard", "altAll"), nil
}

// libGraveToLibrary is Doomsday's head: Origin$ Graveyard,Library to the
// Library with Shuffle$ False, ChangeNum$ N. gorge effSearchLibrary offers
// the library's matches then the graveyard's in one pick; a search that
// states no quality (or Mandatory$) takes min(N, pool) cards (CR 701.23d);
// applyLibrarySearch moves them in answer order and placeLibraryObjects
// puts them on top, the first answered on top, with no shuffle
// (Shuffle$ False). The kernel's SrAltAll search reads the same pool, and
// its SrNoShuffle library destination places the first answered on top
// (moveFoundAns). Only that unshuffled shape: a shuffled put-back would be
// ThenTop over two zones, which the kernel does not model.
func libGraveToLibrary(n lValue, obj lRef, arm bool, flags []string) ([]string, error) {
	switch {
	case !slices.Contains(flags, "NoShuffle"):
		return nil, refuse("Origin$ Library,Graveyard search to Library without Shuffle$ False")
	case slices.Contains(flags, "DifferentNames"):
		return nil, refuse("Origin$ Library,Graveyard search with DifferentNames$")
	case obj.Kind != rYou:
		return nil, refuse("Origin$ Library,Graveyard search with DefinedPlayer$")
	case arm:
		return nil, refuse("Origin$ Library,Graveyard search with WithCountersType$")
	case !n.isConst() || n.N < 1 || n.N > 8:
		return nil, refuse("Origin$ Library,Graveyard search with ChangeNum$ %v", n)
	}
	return append(flags, "alt=Graveyard", "altAll"), nil
}
