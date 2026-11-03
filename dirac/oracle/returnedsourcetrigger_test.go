package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Ygra, Eater of All sees Thought-Stalker Warlock die beside it, but the
// sacrifice of both was the cost of Scavenger's Talent's return ability, which
// then brought Ygra back before any trigger reached the stack. The trigger
// belongs to the Ygra that died (CR 400.7, 603.10a), so the two counters it
// puts on "Ygra" land on nothing, as Forge's CountersPutEffect finds a card
// that changed zones by its game timestamp.
func TestReturnedSourceTriggerFindsNothingLockstep(t *testing.T) {
	t.Parallel()
	stdReproLockstepReaching(t, "testdata/returned_source_trigger.repro.json", "std-blb", func(ref *GorgeSide) bool {
		g := ref.E.G
		for p := range g.Players {
			for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.Obj(id); o != nil && o.Counter("FINALITY") > 0 {
					if f := o.Face(); f != nil && f.Name == "Ygra, Eater of All" {
						return true
					}
				}
			}
		}
		return false
	})
}
