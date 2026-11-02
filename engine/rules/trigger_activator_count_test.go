package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// noTC is the zero trigger provenance: a cast-ledger read outside a trigger.
var noTC effects.TriggerContext

// TestMagebaneLizardCountsTheActivatorsNoncreatureCasts pins the
// Count$ThisTurnCast_Card.nonCreature+ControlledBy TriggeredActivator read end
// to end: the per-cast filter of the cast ledger must see the resolving
// trigger's TriggerActivator, so the damage equals the number of noncreature
// spells the CASTER has cast this turn (1, then 2), not zero.
func TestMagebaneLizardCountsTheActivatorsNoncreatureCasts(t *testing.T) {
	t.Parallel()
	lizard := card(t, "Name:Magebane Lizard\nManaCost:1 R\nTypes:Creature Lizard\nPT:1/4\n"+
		"T:Mode$ SpellCast | TriggerZones$ Battlefield | ValidCard$ Card.nonCreature | ValidActivatingPlayer$ Player | Execute$ TrigDmg | TriggerDescription$ x\n"+
		"SVar:TrigDmg:DB$ DealDamage | Defined$ TriggeredActivator | NumDmg$ Cast\n"+
		"SVar:Cast:Count$ThisTurnCast_Card.nonCreature+ControlledBy TriggeredActivator\nOracle:x\n")
	instant := card(t, "Name:Quick\nManaCost:U\nTypes:Instant\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n")
	e := handEngine(t, lizard, instant, instant)
	hand := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
	liz := hand[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: liz, From: state.ZHand, To: state.ZBattlefield})
	drainQueuedTriggers(t, e)
	hand = hand[1:]
	e.G.Players[0].Pool[state.MU] = 2
	if o := e.G.Obj(liz); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Magebane Lizard is not on the battlefield")
	}
	if e.G.Players[0].Life != 20 {
		t.Fatalf("precondition: life %d, want 20", e.G.Players[0].Life)
	}
	for i, want := range []int32{19, 17} {
		castMode(t, e, hand[i], "")
		drainQueuedTriggers(t, e)
		if got := int32(e.G.Players[0].Life); got != want {
			t.Fatalf("after noncreature cast %d: life %d, want %d", i+1, got, want)
		}
	}
}
