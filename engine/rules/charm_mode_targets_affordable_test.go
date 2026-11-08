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
	const dragon = "Name:Peaks Dragon\nManaCost:1\nTypes:Creature Dragon\nPT:5/4\n" +
		"S:Mode$ RaiseCost | ValidTarget$ Card.Self | Activator$ Player.Opponent | Type$ Spell | Cost$ PayLife<3> | Description$ x\nOracle:x\n"
	const bear = "Name:Peaks Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 6310, charm)
	dragonID := onBoard(t, e, 1, dragon)
	bearID := onBoard(t, e, 1, bear)
	e.G.Players[0].Life = life
	addMana(t, e, 0, "B") // the priority round sees the board and life above
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the mode announcement", d)
	}
	submitChoices(t, e, 0, 1)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 {
		t.Fatalf("pending = %+v, want two per-mode target slots", d)
	}
	return d, dragonID, bearID
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
