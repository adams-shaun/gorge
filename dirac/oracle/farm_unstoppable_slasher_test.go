package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A pinned game in the shape the std-dsk farm game had: Enduring Courage is on
// the battlefield, Unstoppable Slasher enters under it, and the Slasher is
// killed and returned by its own dies trigger while Courage's enters trigger is
// still on the stack. The trigger's Defined$ TriggeredCardLKICopy names the
// Slasher that entered, so the pump does not reach the new object the card
// became (CR 400.7). The pinned actions replay both engines in lockstep.
//
// The condition below is the returned board only: a Slasher with a stun counter
// beside an Enduring Courage. It deliberately does not claim the pending-trigger
// race, which the condition cannot see; the lockstep check is what proves the
// race, and it parts at Cards[].Power when the pump strip is removed.
func TestPinnedFarmUnstoppableSlasher(t *testing.T) {
	pinnedGames(t, "std-dsk", Config{GorgeAutopaySurface: true},
		slasherReturnedUnderCourage,
		onBattlefield("Unstoppable Slasher"))
}

// slasherReturnedUnderCourage is the condition that one player controls an
// Unstoppable Slasher with a stun counter (the object its dies trigger
// returned) beside an Enduring Courage.
func slasherReturnedUnderCourage(ref *GorgeSide) bool {
	g := ref.E.G
	for p := range g.Players {
		slasher, courage := false, false
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			switch o.Face().Name {
			case "Unstoppable Slasher":
				slasher = o.Counter("STUN") > 0
			case "Enduring Courage":
				courage = true
			}
		}
		if slasher && courage {
			return true
		}
	}
	return false
}
