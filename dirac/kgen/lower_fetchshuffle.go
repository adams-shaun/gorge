package kgen

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

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
//
// afterMove is the shuffle a non-empty fetch owes after moving its card.
// afterEmpty is the search's deferred "Then shuffle" that the empty leg must
// still pay (gorge emptyDefinedFetch): it is set only when no later explicit
// DB$ Shuffle in the chain already pays it. Celestial Reunion, Guidelight
// Pathmaker, Archdruid's Charm, Ruin in Their Wake, Treasure Chest and Nazahn
// all close their Shuffle$ False search with DB$ Shuffle, so their empty leg
// must not add a second one; Nissa's Pilgrimage closes with Cleanup, so its
// DBHand empty leg pays.
func (lc *lowerCtx) fetchShuffleOwed(ps *params) (afterMove, afterEmpty bool) {
	explicit := ps.is("Shuffle", "True")
	deferred := false
	for i := len(lc.ab.Prog) - 1; i >= 0; i-- {
		if in := &lc.ab.Prog[i]; in.Op == "Search" {
			_, deferred = flagsOf(in.CS)["NoShuffle"]
			break
		}
	}
	if !explicit && !deferred {
		return false, false
	}
	if !deferred {
		return true, false
	}
	if lc.chainShufflesController() {
		return true, false
	}
	return true, true
}

// markFetchShuffleOwed records fetchShuffleOwed's answer on the fetch
// instruction: ShuffleOwed for the shuffle a non-empty fetch owes, and the
// empty-shuffle marker when the empty leg pays the deferred search shuffle
// itself; the kernel reads that marker (zone_emptyfetchshuffle.go). A later
// explicit DB$ Shuffle pays instead, so the marker is absent.
func (lc *lowerCtx) markFetchShuffleOwed(in *lInstr, ps *params) {
	afterMove, afterEmpty := lc.fetchShuffleOwed(ps)
	if afterMove {
		in.CS += "|ShuffleOwed"
	}
	if afterEmpty {
		in.W = lRef{Kind: rYou}
	}
}

// chainShufflesController reports whether lc.curNode's SubAbility$ chain
// reaches an explicit DB$ Shuffle that randomizes the resolving controller's
// library (gorge chainPaysDeferredShuffle; the corpus tail is Defined$ You or
// no Defined$ at all). That Shuffle pays the deferred search shuffle, so the
// empty fetch leg carries no empty-shuffle marker.
func (lc *lowerCtx) chainShufflesController() bool {
	if lc.curNode == nil {
		return false
	}
	for n := lc.curNode.Sub; n != nil; n = n.Sub {
		if n.APIKind() != cards.APIShuffle {
			continue
		}
		// the same Defined$ reader gorge's chainPaysDeferredShuffle uses
		ref := effects.DefinedRefOf(n)
		if !ref.Set() || ref.Kind == effects.RefYou {
			return true
		}
	}
	return false
}
