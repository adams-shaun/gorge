package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// optionalAskAfter passes until the next optional-trigger ask.
func optionalAskAfter(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 400 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision")
		}
		if d.Kind == decision.KTriggerOptional {
			return d
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	t.Fatal("no optional trigger asked")
	return nil
}

// "That player may": a card names the seat who answers, and it need not be the
// ability's controller (CR 603.5).
func TestOptionalDeciderTriggeredPlayerAsksThatPlayer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := animateTriggersEngine(t, reg)
	onBoard(t, e, 0, "Name:Upkeep Offer\nTypes:Enchantment\n"+
		"T:Mode$ Phase | Phase$ Upkeep | Execute$ Draw | TriggerZones$ Battlefield | OptionalDecider$ TriggeredPlayer | TriggerDescription$ That player may draw.\n"+
		"SVar:Draw:DB$ Draw | Defined$ TriggeredPlayer\nOracle:x\n")
	driveToStep(t, e, 1, 0, state.StepEnd)
	if d := optionalAskAfter(t, e); d.Player != 1 || e.G.Active != 1 {
		t.Fatalf("asked seat %d on seat %d's upkeep, want the upkeep's player", d.Player, e.G.Active)
	}
}

func TestOptionalDeciderAttackingPlayerAsksTheAttacker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := animateTriggersEngine(t, reg, lookup(t, reg, "Curse of Chaos"))
	curse := moveByName(t, e, 0, "Curse of Chaos", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: curse, Player: 0, Text: "attach to player"})
	bears := onBoardReady(t, e, 1, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	driveToStep(t, e, 2, 1, state.StepMain1)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, bears)
	if d := optionalAskAfter(t, e); d.Player != 1 {
		t.Fatalf("asked seat %d, want the attacking player, seat 1", d.Player)
	}
}

func TestOptionalDeciderActivatorAsksTheCaster(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := animateTriggersEngine(t, reg, lookup(t, reg, "Grizzly Bears"))
	onBoard(t, e, 1, "Name:Cast Watcher\nTypes:Enchantment\n"+
		"T:Mode$ SpellCast | ValidActivatingPlayer$ Opponent | Execute$ Draw | TriggerZones$ Battlefield | OptionalDecider$ TriggeredActivator | TriggerDescription$ That player may draw.\n"+
		"SVar:Draw:DB$ Draw | Defined$ TriggeredActivator\nOracle:x\n")
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZHand)
	addMana(t, e, 0, "1G")
	opt := -1
	for _, o := range e.Pending().Options {
		if o.Obj == bears {
			opt = o.Index
		}
	}
	if opt < 0 {
		t.Fatal("no cast option for the bears")
	}
	submitChoices(t, e, opt)
	if d := optionalAskAfter(t, e); d.Player != 0 {
		t.Fatalf("asked seat %d, want the caster, seat 0", d.Player)
	}
}

func TestOptionalDeciderEnchantedControllerAsksThatController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := animateTriggersEngine(t, reg)
	aura := onBoard(t, e, 0, "Name:Upkeep Aura\nTypes:Enchantment Aura\nK:Enchant creature\n"+
		"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ Player.EnchantedController | Execute$ Draw | TriggerZones$ Battlefield | OptionalDecider$ EnchantedController | TriggerDescription$ That player may draw.\n"+
		"SVar:Draw:DB$ Draw | Defined$ TriggeredPlayer\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{bear}})
	driveToStep(t, e, 1, 0, state.StepEnd)
	if d := optionalAskAfter(t, e); d.Player != 1 {
		t.Fatalf("asked seat %d, want the enchanted creature's controller, seat 1", d.Player)
	}
}
