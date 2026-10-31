package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// graveSprinter casts itself from the graveyard, and that cast sets up a one-shot
// entry replacement through a Static$ True trigger.
const graveSprinter = "Name:Grave Sprinter\nManaCost:B\nTypes:Creature Zombie\nPT:2/2\n" +
	"S:Mode$ Continuous | Affected$ Card.Self | EffectZone$ Graveyard | AffectedZone$ Graveyard | MayPlay$ True | Description$ Cast it from your graveyard.\n" +
	"T:Mode$ SpellCast | ValidCard$ Card.Self | ValidSA$ Spell.MayPlaySource | Static$ True | Execute$ MakeBonus\n" +
	"SVar:MakeBonus:DB$ Effect | ExileOnMoved$ Stack | ReplacementEffects$ AddBonus\n" +
	"SVar:AddBonus:Event$ Moved | ValidCard$ Card.EffectSource | Destination$ Battlefield | ReplaceWith$ PutBonus | ReplacementResult$ Updated | Description$ It enters with a counter.\n" +
	"SVar:PutBonus:DB$ PutCounter | Defined$ ReplacedCard | CounterType$ P1P1 | ETB$ True | CounterNum$ 1 | SubAbility$ EndBonus\n" +
	"SVar:EndBonus:DB$ ChangeZone | Defined$ Self | Origin$ Command | Destination$ Exile\n" +
	"Oracle:x\n"

// TestStaticTriggerStaysOffTheStack pins the cast above: the entry bonus is part
// of the permission (CR 614.1c), so the spell is the only stack object, and it
// enters with the counter. A replacement on the board that may ask cannot touch
// a body that makes no replaceable event, so it changes nothing here.
func TestStaticTriggerStaysOffTheStack(t *testing.T) {
	t.Parallel()
	e := handEngine(t)
	onBoard(t, e, 1, "Name:Doubler\nTypes:Enchantment\n"+
		"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidSource$ Card.YouCtrl | ReplaceWith$ Bigger | Description$ Twice the damage.\n"+
		"SVar:Bigger:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ ReplaceCount$DamageAmount/Twice\nOracle:x\n")
	o := e.G.AddObject(card(t, graveSprinter), 0)
	o.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{o.ID})
	e.G.Players[0].Pool[state.MB] = 1
	e.priorityRound()
	submitChoices(t, e, passToCast(t, e, o.ID))
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v, want the spell alone", e.G.Stack)
	}
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(o.ID).Counter("P1P1"); got != 1 {
		t.Fatalf("%d counters, want 1", got)
	}
}
