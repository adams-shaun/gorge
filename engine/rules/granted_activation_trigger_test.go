package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// activationWatcher counts every activation its controller makes, on each of
// the two activation-watching trigger modes.
const activationWatcher = "Name:Activation Watcher\nTypes:Enchantment\n" +
	"T:Mode$ AbilityCast | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ Gain\n" +
	"T:Mode$ SpellAbilityCast | ValidSA$ Activated | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Execute$ Gain\n" +
	"SVar:Gain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"

// watcherPushes counts the triggers the watcher put on the stack or still
// holds in the queue.
func watcherPushes(e *Engine, watcher state.ObjID) int {
	n := countKind(e.L.Events, events.TriggerPush, watcher)
	for _, pt := range e.pendingTriggers {
		if pt.Source == watcher {
			n++
		}
	}
	return n
}

// A granted ability is an activation like a printed one (CR 602.1), so
// "whenever you activate" fires for it whichever kind minted it.
func TestGrantedActivationFiresActivationTriggers(t *testing.T) {
	t.Parallel()
	t.Run("granted by another permanent", func(t *testing.T) {
		grantor := card(t, "Name:Grantor\nManaCost:0\nTypes:Artifact\n"+
			"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddAbility$ Zap\n"+
			"SVar:Zap:AB$ GainLife | Cost$ 0 | LifeAmount$ 1 | Defined$ You | SpellDescription$ Gain.\nOracle:x\n")
		recipient := card(t, "Name:Recipient\nManaCost:0\nTypes:Creature\nPT:1/1\nOracle:x\n")
		e := printedAddabilityEngine(t, grantor, recipient)
		moveCardToBattlefield(t, e, grantor)
		id := moveCardToBattlefield(t, e, recipient)
		watcher := onBoard(t, e, 0, activationWatcher)
		addMana(t, e, 0, "")
		opt, ok := findSVarOption(t, e, id, "Zap")
		if !ok {
			t.Fatalf("granted ability not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, opt.Index)
		if n := countKind(e.L.Events, events.GrantAbilityPush, id); n != 1 {
			t.Fatalf("GrantAbilityPush count = %d, want 1", n)
		}
		if got := watcherPushes(e, watcher); got != 2 {
			t.Fatalf("watcher triggers = %d, want one per mode", got)
		}
	})
	t.Run("granted to itself", func(t *testing.T) {
		self := card(t, "Name:Self Granter\nManaCost:0\nTypes:Artifact\n"+
			"S:Mode$ Continuous | Affected$ Card.Self | AddAbility$ Zap\n"+
			"SVar:Zap:AB$ GainLife | Cost$ 0 | LifeAmount$ 1 | Defined$ You | SpellDescription$ Gain.\nOracle:x\n")
		e := printedAddabilityEngine(t, self)
		id := moveCardToBattlefield(t, e, self)
		watcher := onBoard(t, e, 0, activationWatcher)
		addMana(t, e, 0, "")
		opt, ok := findSVarOption(t, e, id, "Zap")
		if !ok {
			t.Fatalf("self-granted ability not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, opt.Index)
		if n := countKind(e.L.Events, events.DelayedPush, id); n != 1 {
			t.Fatalf("DelayedPush count = %d, want 1", n)
		}
		if got := watcherPushes(e, watcher); got != 2 {
			t.Fatalf("watcher triggers = %d, want one per mode", got)
		}
	})
	t.Run("gained from a foreign card", func(t *testing.T) {
		welder := tokenReplCorpusCard(t, "Myr Welder")
		vault := tokenReplCorpusCard(t, "Knowledge Vault")
		e, _ := tokenReplGame(t, 9131, welder, vault)
		welderID := moveSeededCard(t, e, 0, welder, state.ZBattlefield)
		vaultID := moveSeededCard(t, e, 0, vault, state.ZGraveyard)
		e.priorityRound()
		gainsDriveToStep(t, e, 3, 0, state.StepMain1)
		imprint := -1
		for _, o := range e.Pending().Options {
			if o.Kind == "ability" && o.Obj == welderID && o.GainedSource == 0 {
				imprint = o.Index
			}
		}
		if imprint < 0 {
			t.Fatalf("no imprint ability offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, imprint)
		for _, o := range e.Pending().Options {
			if o.Obj == vaultID {
				submitChoices(t, e, o.Index)
				break
			}
		}
		passUntilStackEmpty(t, e, 20)
		watcher := onBoard(t, e, 0, activationWatcher)
		e.priorityRound()
		gained := -1
		for _, o := range e.Pending().Options {
			if o.Kind == "ability" && o.Obj == welderID && o.GainedSource == vaultID {
				gained = o.Index
			}
		}
		if gained < 0 {
			t.Fatalf("gained ability not offered: %+v", e.Pending().Options)
		}
		submitChoices(t, e, gained)
		if n := countKind(e.L.Events, events.GainedAbilityPush, welderID); n != 1 {
			t.Fatalf("GainedAbilityPush count = %d, want 1", n)
		}
		if got := watcherPushes(e, watcher); got != 2 {
			t.Fatalf("watcher triggers = %d, want one per mode", got)
		}
	})
}

// A loyalty ability granted to a planeswalker is a loyalty ability for a
// ValidSA$ narrowing, read from the grantor's body.
func TestGrantedLoyaltyAbilityMatchesValidSA(t *testing.T) {
	t.Parallel()
	grantor := card(t, "Name:Loyalty Grantor\nManaCost:0\nTypes:Enchantment\n"+
		"S:Mode$ Continuous | Affected$ Planeswalker.YouCtrl | AddAbility$ Up\n"+
		"SVar:Up:AB$ GainLife | Cost$ AddCounter<1/LOYALTY> | Planeswalker$ True | LifeAmount$ 1 | Defined$ You | SpellDescription$ Gain.\nOracle:x\n")
	walker := card(t, "Name:Fixture Walker\nManaCost:0\nTypes:Legendary Planeswalker Fixture\nLoyalty:3\nOracle:x\n")
	e := printedAddabilityEngine(t, grantor, walker)
	moveCardToBattlefield(t, e, grantor)
	id := moveCardToBattlefield(t, e, walker)
	loyalty := onBoard(t, e, 0, "Name:Loyalty Watcher\nTypes:Enchantment\n"+
		"T:Mode$ AbilityCast | ValidSA$ Activated.Loyalty | TriggerZones$ Battlefield | Execute$ Gain\n"+
		"SVar:Gain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n")
	other := onBoard(t, e, 0, "Name:Other Watcher\nTypes:Enchantment\n"+
		"T:Mode$ AbilityCast | ValidSA$ Activated.!Loyalty | TriggerZones$ Battlefield | Execute$ Gain\n"+
		"SVar:Gain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n")
	addMana(t, e, 0, "")
	opt, ok := findSVarOption(t, e, id, "Up")
	if !ok {
		t.Fatalf("granted loyalty ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	if got := watcherPushes(e, loyalty); got != 1 {
		t.Errorf("loyalty watcher triggers = %d, want 1", got)
	}
	if got := watcherPushes(e, other); got != 0 {
		t.Errorf("non-loyalty watcher triggers = %d, want 0", got)
	}
}

// A delayed trigger's push is a DelayedPush too, and it is no activation.
func TestDelayedTriggerPushIsNotAnActivation(t *testing.T) {
	t.Parallel()
	for _, ev := range []events.Event{
		{Kind: events.DelayedPush, Amount: 3, Counter: "Body", Text: "delayed trigger"},
		{Kind: events.DelayedPush, Amount: 3, Counter: "Body"},
	} {
		if ev.IsActivationMint() {
			t.Errorf("%+v reads as an activation", ev)
		}
	}
}
