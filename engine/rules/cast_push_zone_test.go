package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// beamWatcher is Vash, Freelance Archaeologist's and Hologram Stalker's
// "whenever a card leaves your graveyard ... This ability triggers only once
// each turn", gaining life so the test reads it off the life total.
const beamWatcher = "Name:Grave Watcher\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n" +
	"T:Mode$ ChangesZone | Origin$ Graveyard | Destination$ Any | TriggerZones$ Battlefield | ValidCard$ Card.YouOwn | Execute$ TrigGain | ActivationLimit$ 1 | TriggerDescription$ x\n" +
	"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 5\nOracle:x\n"

// TestCastFromGraveyardTriggersAWatcherTheCostReturns: a beam me up spell
// leaves the graveyard as it is put on the stack (CR 601.2a), before its cost
// returns a creature (601.2h). A watcher on the battlefield at the move
// triggers, whichever creature the cost returns, and its ability goes on the
// stack though its source has left (CR 603.3, 113.7a).
func TestCastFromGraveyardTriggersAWatcherTheCostReturns(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		pick   int
		inHand int // 0 the watcher, 1 the Bear
	}{{"watcher returned", 0, 0}, {"other creature returned", 1, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg, spell := beamFixture(t, 803, beamSpell, beamWatcher, beamBear)
			ids := []state.ObjID{putCreature(t, e, 0, beamWatcher), putCreature(t, e, 0, beamBear)}
			e.Advance()
			opts := beamOptions(t, e, spell)
			if len(opts) != 1 {
				t.Fatalf("beam me up options = %+v, want one", opts)
			}
			life := e.G.Players[0].Life
			submitChoices(t, e, opts[0].Index)
			submitChoices(t, e, tc.pick)
			if z := e.G.Obj(ids[tc.inHand]).Zone; z != state.ZHand {
				t.Fatalf("returned creature is in %s, want hand", z)
			}
			if got := len(e.G.Stack); got != 2 {
				t.Fatalf("stack holds %d objects, want the spell and the watcher's trigger", got)
			}
			resolveStack(t, e)
			if got := e.G.Players[0].Life; got != life+3+5 {
				t.Fatalf("life %d, want %d (the spell's 3 and the trigger's 5)", got, life+8)
			}
			replayCheck(t, e, cfg)
		})
	}
}
