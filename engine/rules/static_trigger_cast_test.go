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

// TestStaticTriggerEventBodyStack pins CR 614.1c for bodies that do make a
// replaceable event: only a replacement of that very event, one that can ask, keeps
// the trigger on the stack, where a resolution can serve the ask. Off the stack
// there is nothing to resume, so that one case stays an ability.
func TestStaticTriggerEventBodyStack(t *testing.T) {
	t.Parallel()
	const doubler = "Name:Doubler\nTypes:Enchantment\n" +
		"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidSource$ Card.YouCtrl | ReplaceWith$ Bigger | Description$ Twice the damage.\n" +
		"SVar:Bigger:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ ReplaceCount$DamageAmount/Twice\nOracle:x\n"
	counterBonus := func(name string) string {
		return "Name:" + name + "\nTypes:Enchantment\n" +
			"R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Card | ReplaceWith$ More | Description$ One more counter.\n" +
			"SVar:More:DB$ ReplaceCounter | ValidCounterType$ Any | Amount$ Bumped\nSVar:Bumped:ReplaceCount$CounterNum/Plus.1\nOracle:x\n"
	}
	const choosyDraw = "Name:Choosy Draw\nTypes:Enchantment\n" +
		"R:Event$ Draw | ActiveZones$ Battlefield | Optional$ True | ValidPlayer$ You | ReplaceWith$ Instead | Description$ You may gain 1 instead.\n" +
		"SVar:Instead:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	const spark = "Name:Spark\nManaCost:R\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ Gain 1.\nOracle:x\n"
	watcher := func(body string) string {
		return "Name:Watcher\nTypes:Creature Human\nPT:1/1\n" +
			"T:Mode$ SpellCast | ValidCard$ Card | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Static$ True | Execute$ Body\n" +
			"SVar:Body:" + body + "\nOracle:x\n"
	}
	const grow = "DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1"
	const burn = "DB$ DealDamage | Defined$ Opponent | NumDmg$ 1"
	for _, c := range []struct {
		name, body string
		board      []string
		wantStack  int
	}{
		{"a counter beside another event's replacement", grow, []string{doubler}, 1},
		{"a counter beside its own event's asking replacement", grow, []string{counterBonus("Bonus")}, 2},
		{"a counter between two bonuses", grow, []string{counterBonus("Bonus"), counterBonus("Other Bonus")}, 2},
		{"damage beside another event's replacement", burn, []string{choosyDraw}, 1},
		{"damage beside its own event's asking replacement", burn, []string{doubler}, 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, spark))
			id := e.G.Zone(state.ZHand, 0)[0]
			onBoardReady(t, e, 0, watcher(c.body))
			for _, src := range c.board {
				onBoard(t, e, 1, src)
			}
			e.G.Players[0].Pool[state.MR] = 1
			e.priorityRound()
			submitChoices(t, e, passToCast(t, e, id))
			if len(e.G.Stack) != c.wantStack {
				t.Fatalf("stack %v, want %d objects", e.G.Stack, c.wantStack)
			}
		})
	}
}
