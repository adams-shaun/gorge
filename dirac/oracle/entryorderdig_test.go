package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// CR 616.1: a land put onto the battlefield by a dig enters under The Warp Speed
// enchantment's "Lands you control enter untapped" and its own enters-tapped body
// (a shock land, a tapped planet or town land). gorge asks the order on the move; the
// kernel poses the same ask with the card still in the library, on a DigUntil's found
// card (U.S.S. Voyager, Lone Wanderer) and on a Dig's taken card (Hit It!). A shock
// land's election still rides the order: a body ordered before the untap leaves it
// untapped, the move's Tapped$ taps no later than the bodies.

// orderAskWhileResolving: a CR 616.1 order ask is pending with an object on the stack,
// so a resolving effect (not a land play) moves the land.
func orderAskWhileResolving(ref *GorgeSide) bool {
	d := ref.Pending()
	return d != nil && d.Kind == decision.KReplacement && len(ref.E.G.Stack) > 0
}

func TestEntryOrderDigUntilLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	pinnedGames(t, "std-trk", Config{GorgeAutopaySurface: true},
		onBattlefield("Warp Speed"), onBattlefield("U.S.S. Voyager, Lone Wanderer"), orderAskWhileResolving)
}

func TestEntryOrderDigLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	pinnedGames(t, "std-trk", Config{GorgeAutopaySurface: true},
		onBattlefield("Warp Speed"), onBattlefield("Hit It!"), orderAskWhileResolving)
}

// A library search puts several lands onto the battlefield one at a time (Genesis
// Device): each one that meets the same competition is asked in turn.
func TestEntryOrderSearchLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	pinnedGames(t, "std-trk", Config{GorgeAutopaySurface: false},
		onBattlefield("Warp Speed"), onBattlefield("Genesis Device"), orderAskWhileResolving)
}

// A mass move onto the battlefield (Deep Space 9 returns the exiled lands) asks each
// competing land in turn.
func TestEntryOrderMassMoveLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	pinnedGames(t, "std-trk", Config{GorgeAutopaySurface: true},
		onBattlefield("Warp Speed"), onBattlefield("Deep Space 9"), orderAskWhileResolving)
}
