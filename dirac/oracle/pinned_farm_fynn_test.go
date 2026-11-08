package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmFynntheFangbearer replays the std-fdn farm game that diverged
// with Fynn, the Fangbearer implicated (xe:RAND:3:16, turn 18): the UB seat's
// Painful Quandary made the WG seat discard Wilt-Leaf Liege, and the
// opponent-caused discard replacement (CR 614.1) put the Liege onto the
// battlefield in gorge while dirac put it into the graveyard, so every other
// green and white creature lost its +1/+1. The recorded decks are the farm
// game's, in the decklist order that deals its opening at the seed; the actions
// are a fixed prefix that reaches Fynn on the battlefield and the discard
// replacement. Both must hold, and the lockstep must stay clean.
func TestPinnedFarmFynntheFangbearer(t *testing.T) {
	t.Parallel()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	pinnedGamesRun(t, "std-fdn", rc, 0,
		onBattlefield("Fynn, the Fangbearer"), discardedIntoPlay("Wilt-Leaf Liege"))
}

// discardedIntoPlay is the condition that a card of the given name was put
// onto the battlefield by a discard move rather than cast (CR 614.1).
func discardedIntoPlay(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		evs := ref.E.L.Events
		for i := range evs {
			e := &evs[i]
			if e.Kind != events.MoveZone || e.To != state.ZBattlefield || !events.IsDiscard(*e) {
				continue
			}
			if o := ref.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
}
