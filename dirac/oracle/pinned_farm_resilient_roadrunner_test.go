package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmResilientRoadrunner replays the std-otj farm game that diverged with
// Resilient Roadrunner implicated (OTJ-s266, xe:RAND:1:12, turn 12): the WR seat
// activated the Roadrunner's "can't be blocked except by creatures with haste" with
// Nurturing Pixie's enters trigger already on the stack, the trigger bounced it, and
// the recast Roadrunner attacked. The effect lasts while that host stays on the
// battlefield (Duration$ UntilHostLeavesPlayOrEOT), and the returned card is a new
// object (CR 400.7), so the opponent's untapped Oasis Gardener may block it: dirac
// asked for blockers and gorge, which let the old effect govern the new object, went
// straight to the next priority. The decks are the farm's, in the decklist order
// (sorted by name) that deals its recorded hands and libraries at the game seed
// 266*1000003+1*10007+12; the actions are the game's, played by the explore policy on
// the autopay surface, up to 150 decisions past the recast attack. Both conditions
// must hold, and the lockstep must stay clean.
func TestPinnedFarmResilientRoadrunner(t *testing.T) {
	t.Parallel()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	pinnedGamesRun(t, "std-otj", rc, 0, roadrunnerBouncedThenAttacks)
}

// roadrunnerBouncedThenAttacks is the condition that a Resilient Roadrunner's ability
// was activated, the card then went from the battlefield to a hand, and the same card
// later was declared as an attacker.
func roadrunnerBouncedThenAttacks(ref *GorgeSide) bool {
	var activated, bounced bool
	evs := ref.E.L.Events
	var rr state.ObjID
	for i := range evs {
		e := &evs[i]
		o := ref.E.G.Obj(e.Obj)
		isRR := o != nil && o.Face() != nil && o.Face().Name == "Resilient Roadrunner"
		switch {
		case e.Kind == events.AbilityPush && isRR && !activated:
			activated, rr = true, e.Obj
		case e.Kind == events.MoveZone && activated && e.Obj == rr && e.From == state.ZBattlefield && e.To == state.ZHand:
			bounced = true
		case e.Kind == events.DeclareAttackers && bounced:
			for _, id := range e.IDs {
				if id == rr {
					return true
				}
			}
			if e.Obj == rr {
				return true
			}
		}
	}
	return false
}
