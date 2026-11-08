package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A game recorded from the std-dsk farm where Unstoppable Slasher returned
// from its own dies trigger while Enduring Courage's enters trigger was still
// on the stack. The trigger's Defined$ TriggeredCardLKICopy names the Slasher
// that entered, so the pump does not reach the new object the card became
// (CR 400.7). The pinned actions replay both engines in lockstep.
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
