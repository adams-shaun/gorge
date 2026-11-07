package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// upToZeroTriggerSrc is a creature whose enters trigger is a Charm "choose up
// to X" (MinCharmNum$ 0) with X = Count$Kicked.2.0, which is 0 on a plain cast.
const upToZeroTriggerSrc = "Name:Up To Zero Trigger Probe\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCharm | TriggerDescription$ Choose up to X.\n" +
	"SVar:TrigCharm:DB$ Charm | MinCharmNum$ 0 | CharmNum$ X | Choices$ DBOne,DBTwo\n" +
	"SVar:DBOne:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\n" +
	"SVar:DBTwo:DB$ GainLife | Defined$ You | LifeAmount$ 2 | SpellDescription$ Gain 2 life.\n" +
	"SVar:X:Count$Kicked.2.0\nOracle:x\n"

// TestCharmTriggerUpToZeroIsRemoved pins CR 700.2a + CR 603.3c: a modal
// trigger whose count is 0 at placement chooses no mode and is removed, with
// no mode ask posed.
func TestCharmTriggerUpToZeroIsRemoved(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 33, upToZeroTriggerSrc)
	addMana(t, e, 0, "R")
	idx := -1
	for _, o := range castOptions(t, e) {
		if o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option: %+v", castOptions(t, e))
	}
	submitChoices(t, e, idx)
	entered := false
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KModes {
			t.Fatalf("a count of 0 posed a mode ask: %+v", d)
		}
		if entered && len(e.G.Stack) == 0 {
			break
		}
		pass := d.Options[0].Index
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		submitChoices(t, e, pass)
		if len(e.G.Stack) == 0 {
			entered = true
		}
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("the up-to-zero trigger stayed on the stack: %v", e.G.Stack)
	}
	if o := e.G.Obj(id); !entered || o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("the creature spell never resolved onto the battlefield")
	}
}
