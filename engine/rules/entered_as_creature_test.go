package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// "Another creature entered this turn" reads what entered (Forge stores the entry's
// state, XMage's watcher the entering permanent): a land animated afterwards does not
// meet it, and a creature entering does.
func TestThisTurnEnteredCreatureReadsTheEntry(t *testing.T) {
	t.Parallel()
	const outrider = "Name:Entry Outrider\nManaCost:1\nTypes:Creature Knight\nPT:3/5\n" +
		"S:Mode$ Continuous | AffectedDefined$ Self | AddPower$ 2 | CheckSVar$ X | Description$ x\n" +
		"SVar:X:Count$ThisTurnEntered_Battlefield_Creature.StrictlyOther+YouCtrl\nOracle:x\n"
	const land = "Name:Entry Land\nTypes:Land\nA:AB$ Animate | Cost$ 0 | Defined$ Self | Power$ 2 | Toughness$ 2 | Types$ Creature | " +
		"SpellDescription$ x\nOracle:x\n"
	const bear = "Name:Entry Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e := handEngine(t, card(t, outrider), card(t, land), card(t, bear))
	hand := e.G.Zone(state.ZHand, 0)
	knight, field, grizzly := hand[0], hand[1], hand[2]
	placeOnBattlefield(t, e, knight)
	placeOnBattlefield(t, e, field)
	ppPriority(t, e)
	ppChoose(t, e, func(o decision.Option) bool { return o.Kind == "ability" && o.Obj == field })
	drainQueuedTriggers(t, e)
	if !e.IsCreature(field) {
		t.Fatal("precondition: the land is not a creature")
	}
	if got := e.Power(knight); got != 3 {
		t.Fatalf("power %d with only an animated land entered, want 3", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: grizzly, From: state.ZHand, To: state.ZBattlefield})
	if got := e.Power(knight); got != 5 {
		t.Fatalf("power %d after a creature entered, want 5", got)
	}
}
