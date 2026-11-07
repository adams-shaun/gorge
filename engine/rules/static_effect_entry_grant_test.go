package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestStaticEffectGrantAppliesBeforeEntryTriggersMatch: a StaticEffect$ rider
// on a return to the battlefield is part of the permanent an "enters" trigger
// looks at (CR 603.6a, 614.12): a returned Enduring creature that is no longer
// a creature does not trigger "another creature enters", and a granted type
// does.
func TestStaticEffectGrantAppliesBeforeEntryTriggersMatch(t *testing.T) {
	t.Parallel()
	clown := "Name:Fixture Clown\nManaCost:2 R\nTypes:Creature Human Clown\nPT:2/3\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.powerLE2+YouCtrl+Other | TriggerZones$ Battlefield | Execute$ FxGrow | TriggerDescription$ x\n" +
		"SVar:FxGrow:DB$ Pump | Defined$ Self | NumAtt$ +2\nOracle:x\n"
	sheep := "Name:Fixture Sheep\nManaCost:1 W W\nTypes:Enchantment Creature Sheep\nPT:2/1\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+Creature | Execute$ FxBack | TriggerDescription$ x\n" +
		"SVar:FxBack:DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | StaticEffect$ FxShape\n" +
		"SVar:FxShape:Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Enchantment | RemoveCardTypes$ True\nOracle:x\n"
	angelClown := "Name:Fixture Watcher\nManaCost:2 R\nTypes:Creature Human\nPT:2/3\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Angel.YouCtrl+Other | TriggerZones$ Battlefield | Execute$ FxGrow | TriggerDescription$ x\n" +
		"SVar:FxGrow:DB$ Pump | Defined$ Self | NumAtt$ +2\nOracle:x\n"
	relic := "Name:Fixture Relic\nManaCost:2\nTypes:Creature Sheep\nPT:2/1\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ FxBack | TriggerDescription$ x\n" +
		"SVar:FxBack:DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | StaticEffect$ FxShape\n" +
		"SVar:FxShape:Mode$ Continuous | Affected$ Card.IsRemembered | AddType$ Angel\nOracle:x\n"

	drain := func(t *testing.T, e *Engine) {
		t.Helper()
		for i := 0; i < 40; i++ {
			d := e.Pending()
			if d == nil || (d.Kind == decision.KPriority && len(e.G.Stack) == 0) {
				break
			}
			if d.Kind == decision.KPriority {
				passFirst(t, e)
			} else {
				var idx []int
				for _, o := range d.Options {
					idx = append(idx, o.Index)
				}
				submitChoices(t, e, idx...)
			}
		}
	}
	run := func(t *testing.T, watcher, src string) (*Engine, Config, state.ObjID, state.ObjID) {
		e, cfg, _ := newFixtureDeck(t, 6311, watcher, src)
		cid := putCreature(t, e, 0, watcher)
		id := putCreature(t, e, 0, src)
		e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 5, Player: 0})
		e.checkStateBased()
		e.putTriggersOnStack()
		if e.Pending() == nil {
			e.priorityRound()
		}
		drain(t, e)
		return e, cfg, cid, id
	}
	has := func(e *Engine, id state.ObjID, typ string) bool {
		for _, ty := range e.Derived(id).Types {
			if ty == typ {
				return true
			}
		}
		return false
	}

	t.Run("removed creature type does not trigger", func(t *testing.T) {
		e, cfg, cid, id := run(t, clown, sheep)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("sheep did not return: %+v", o)
		}
		if has(e, id, "Creature") || !has(e, id, "Enchantment") {
			t.Fatalf("returned types = %v, want a non-creature Enchantment", e.Derived(id).Types)
		}
		if p := e.Power(cid); p != 4 {
			t.Fatalf("clown power = %d, want %d (the first entry pumped; the non-creature return must not)", p, 4)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("added creature type triggers", func(t *testing.T) {
		e, cfg, cid, id := run(t, angelClown, relic)
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("relic did not return: %+v", o)
		}
		if !has(e, id, "Angel") {
			t.Fatalf("returned types = %v, want Angel", e.Derived(id).Types)
		}
		if p := e.Power(cid); p != 4 {
			t.Fatalf("clown power = %d, want %d (2 + the granted Angel's entry)", p, 4)
		}
		replayCheck(t, e, cfg)
	})
}
