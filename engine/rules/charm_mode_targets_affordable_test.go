package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// charmModeTargetMenu casts a two-mode Charm (pump target creature, target
// player loses 1 life) at the given caster life with an opponent's "spells
// your opponents cast that target this cost an additional 3 life" Dragon and
// a plain Bear on the table, and returns the per-mode target decision and
// the two creatures.
func charmModeTargetMenu(t *testing.T, life int32) (*decision.Decision, state.ObjID, state.ObjID) {
	t.Helper()
	const charm = "Name:Peaks Charm\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 2 | Choices$ Pump,Drain\n" +
		"SVar:Pump:DB$ Pump | ValidTgts$ Creature | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ Target creature gets +1/+1.\n" +
		"SVar:Drain:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 1 | SpellDescription$ Target player loses 1 life.\n" +
		"Oracle:x\n"
	return modeTargetMenu(t, charm, "B", life, 0)
}

// spreeModeTargetMenu is charmModeTargetMenu for a Spree spell whose two
// modes add {2} and {1}: with {B} floating and two untapped Swamps each mode
// alone is payable but both ({3}{B}) are not, so the announced mode set is
// mana-short before any target is chosen (CR 601.2h reverses it at payment).
func spreeModeTargetMenu(t *testing.T, life int32) (*decision.Decision, state.ObjID, state.ObjID) {
	t.Helper()
	const spree = "Name:Peaks Spree\nManaCost:B\nTypes:Instant\nK:Spree\n" +
		"A:SP$ Charm | MinCharmNum$ 1 | CharmNum$ 2 | Choices$ Pump,Drain\n" +
		"SVar:Pump:DB$ Pump | ModeCost$ 2 | ValidTgts$ Creature | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ Target creature gets +1/+1.\n" +
		"SVar:Drain:DB$ LoseLife | ModeCost$ 1 | ValidTgts$ Player | LifeAmount$ 1 | SpellDescription$ Target player loses 1 life.\n" +
		"Oracle:x\n"
	d, dragonID, bearID, e := modeTargetMenuEngine(t, spree, "B", life, 2)
	pr, ok := e.newTargetCostProbe(e.cast)
	if !ok || pr.verdict(nil, nil, nil) != probeManaShort {
		t.Fatalf("fixture: the announced Spree mode set must be mana-short before any target (ok=%v)", ok)
	}
	return d, dragonID, bearID
}

func modeTargetMenu(t *testing.T, charm, mana string, life int32, swamps int) (*decision.Decision, state.ObjID, state.ObjID) {
	d, dragonID, bearID, _ := modeTargetMenuEngine(t, charm, mana, life, swamps)
	return d, dragonID, bearID
}

func modeTargetMenuEngine(t *testing.T, charm, mana string, life int32, swamps int) (*decision.Decision, state.ObjID, state.ObjID, *Engine) {
	t.Helper()
	const dragon = "Name:Peaks Dragon\nManaCost:1\nTypes:Creature Dragon\nPT:5/4\n" +
		"S:Mode$ RaiseCost | ValidTarget$ Card.Self | Activator$ Player.Opponent | Type$ Spell | Cost$ PayLife<3> | Description$ x\nOracle:x\n"
	const bear = "Name:Peaks Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 6310, charm)
	dragonID := onBoard(t, e, 1, dragon)
	bearID := onBoard(t, e, 1, bear)
	for i := 0; i < swamps; i++ {
		onBoard(t, e, 0, "Name:Peaks Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
	}
	e.G.Players[0].Life = life
	addMana(t, e, 0, mana) // the priority round sees the board and life above
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the mode announcement", d)
	}
	submitChoices(t, e, 0, 1)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 {
		t.Fatalf("pending = %+v, want two per-mode target slots", d)
	}
	return d, dragonID, bearID, e
}

func modeOffers(d *decision.Decision, group string, obj state.ObjID) bool {
	for _, o := range d.Options {
		if o.Group == group && o.Obj == obj {
			return true
		}
	}
	return false
}

// TestCharmModeTargetsWithholdUnaffordableRaise: a per-mode Charm target ask
// withholds a creature whose "an additional 3 life" the caster cannot pay
// (CR 601.2c/f, 118.8: Terror of the Peaks), as the root and SubAbility$
// link asks do; at a life total that pays it the creature stays offered.
func TestCharmModeTargetsWithholdUnaffordableRaise(t *testing.T) {
	t.Parallel()
	d, dragonID, bearID := charmModeTargetMenu(t, 20)
	if !modeOffers(d, "charm-mode-0", dragonID) || !modeOffers(d, "charm-mode-0", bearID) {
		t.Fatalf("at 20 life both creatures must be legal, affordable targets: %+v", d.Options)
	}
	d, dragonID, bearID = charmModeTargetMenu(t, 2)
	if !modeOffers(d, "charm-mode-0", bearID) {
		t.Fatalf("at 2 life the Bear (no added cost) must stay offered: %+v", d.Options)
	}
	if modeOffers(d, "charm-mode-0", dragonID) {
		t.Fatalf("at 2 life the Dragon's 3-life raise is unpayable, yet it is offered: %+v", d.Options)
	}
}

// TestSpreeModeTargetsKeepRaiseWhenModesManaShort: when the announced Spree
// mode set is already mana-short, a target is withheld only for the part of
// the cost IT makes unpayable (CR 601.2c/f): the Dragon's 3 life at 20 life
// is payable, so it stays offered (the payment reverses the cast, CR
// 601.2h); at 2 life it is withheld.
func TestSpreeModeTargetsKeepRaiseWhenModesManaShort(t *testing.T) {
	t.Parallel()
	d, dragonID, bearID := spreeModeTargetMenu(t, 20)
	if !modeOffers(d, "charm-mode-0", dragonID) || !modeOffers(d, "charm-mode-0", bearID) {
		t.Fatalf("a mana-short mode set at 20 life must keep both creatures: %+v", d.Options)
	}
	d, dragonID, bearID = spreeModeTargetMenu(t, 2)
	if !modeOffers(d, "charm-mode-0", bearID) {
		t.Fatalf("at 2 life the Bear (no added cost) must stay offered: %+v", d.Options)
	}
	if modeOffers(d, "charm-mode-0", dragonID) {
		t.Fatalf("at 2 life the Dragon's 3-life raise is unpayable, yet it is offered: %+v", d.Options)
	}
}
