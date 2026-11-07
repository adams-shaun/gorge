package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLeavesGraveyardTriggerLooksBackAtTheCreatureCard: "whenever a creature
// card leaves your graveyard" looks back in time (CR 603.10a). A creature
// card that dies and returns as a noncreature artifact (a StaticEffect$
// rider that removes its card types) was a creature card in the graveyard,
// so the watcher triggers on its return. The fixtures are written from the
// Oracle text.
func TestLeavesGraveyardTriggerLooksBackAtTheCreatureCard(t *testing.T) {
	t.Parallel()
	watcher := "Name:Fixture Grave Watcher\nManaCost:2 B\nTypes:Enchantment\n" +
		"T:Mode$ ChangesZone | Origin$ Graveyard | Destination$ Any | TriggerZones$ Battlefield | ValidCard$ Creature.YouOwn | Execute$ FxGain | TriggerDescription$ x\n" +
		"SVar:FxGain:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n"
	troll := "Name:Fixture Returning Troll\nManaCost:3 B G\nTypes:Creature Troll\nPT:5/5\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+Creature | Execute$ FxBack | TriggerDescription$ x\n" +
		"SVar:FxBack:DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | StaticEffect$ FxShape\n" +
		"SVar:FxShape:Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Artifact | RemoveCardTypes$ True\nOracle:x\n"

	e, cfg, _ := newFixtureDeck(t, 6312, watcher, troll)
	putCreature(t, e, 0, watcher)
	id := putCreature(t, e, 0, troll)
	life := e.G.Players[0].Life
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 5, Player: 0})
	e.checkStateBased()
	e.putTriggersOnStack()
	if e.Pending() == nil {
		e.priorityRound()
	}
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || (d.Kind == decision.KPriority && len(e.G.Stack) == 0) {
			break
		}
		if d.Kind == decision.KPriority {
			passFirst(t, e)
			continue
		}
		var idx []int
		for _, o := range d.Options {
			idx = append(idx, o.Index)
		}
		submitChoices(t, e, idx...)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the troll did not return: %+v", o)
	}
	for _, ty := range e.Derived(id).Types {
		if ty == "Creature" {
			t.Fatalf("precondition: the returned troll is still a creature: %v", e.Derived(id).Types)
		}
	}
	if got := e.G.Players[0].Life - life; got != 3 {
		t.Fatalf("life gained = %d, want 3: the creature card leaving the graveyard must trigger the watcher", got)
	}
	replayCheck(t, e, cfg)
}
