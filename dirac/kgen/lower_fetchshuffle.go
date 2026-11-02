package kgen

// fetchShuffleOwed reports whether a Defined$ library fetch (gorge
// moveDefinedLibraryObjects) owes its owner's library a shuffle. The fetch
// is not a search, so the implicit shuffle fires only for an explicit
// Shuffle$ True, a library destination, or a fetch that finishes a search
// whose shuffle was deferred: gorge sets Search.DeferredShuffle on every
// library search (Shuffle$ False or NoShuffle$ True without Shuffle$ True),
// so the NEAREST preceding library search in the chain decides (zone_search.go
// implicitShuffle, applyLibrarySearch; Archdruid's Charm, Guidelight
// Pathmaker). A fetch with no search before it (Traveling Botanist, Gathering
// Stone, Bucolic Ranch, Archghoul of Thraben) owes nothing. Only Bounce and
// Return reach here, so the library destination arm does not apply.
func (lc *lowerCtx) fetchShuffleOwed(ps *params) bool {
	if ps.is("Shuffle", "True") {
		return true
	}
	for i := len(lc.ab.Prog) - 1; i >= 0; i-- {
		if in := &lc.ab.Prog[i]; in.Op == "Search" {
			_, deferred := flagsOf(in.CS)["NoShuffle"]
			return deferred
		}
	}
	return false
}
