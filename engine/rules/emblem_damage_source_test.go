package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 114.2: an emblem is owned and controlled by the player who gets it, and
// CR 114.3: it has no characteristics but its abilities. So the damage an
// emblem's ability deals is from a source that player controls, never from
// the card whose resolution made the emblem.

// emblemMakerSrc is a red lifelinking permanent whose ability gives each
// opponent an emblem: "At the beginning of your upkeep, this emblem deals 1
// damage to you and 1 damage to each creature you control."
const emblemMakerSrc = "Name:Test Inferno\nManaCost:4 R R\nTypes:Creature Elemental\nPT:6/6\nK:Lifelink\n" +
	"A:AB$ Effect | Cost$ T | EffectOwner$ Player.Opponent | Name$ Emblem — Test Inferno | Triggers$ BOTTrig | Duration$ Permanent\n" +
	"SVar:BOTTrig:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Command | Execute$ Burn\n" +
	"SVar:Burn:DB$ DealDamage | Defined$ TriggeredPlayer | NumDmg$ 1 | SubAbility$ Singe\n" +
	"SVar:Singe:DB$ DamageAll | NumDmg$ 1 | ValidCards$ Creature.YouCtrl\n" +
	"Oracle:x\n"

// emblemPitSrc is Torture Pit's line: +2 noncombat damage from a source (or
// emblem) its controller controls to an opponent.
const emblemPitSrc = "Name:Test Torture Pit\nManaCost:3 R\nTypes:Enchantment\n" +
	"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidSource$ Card.YouCtrl,Emblem.YouCtrl | ValidTarget$ Opponent | IsCombat$ False | ReplaceWith$ DamageReplace\n" +
	"SVar:DamageReplace:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ ReplaceCount$DamageAmount/Plus.2\n" +
	"Oracle:x\n"

// emblemOwnPitSrc is the same +2 line pointed at its controller, so it
// applies exactly when the source is one that player controls.
const emblemOwnPitSrc = "Name:Test Self Pit\nManaCost:3 R\nTypes:Enchantment\n" +
	"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidSource$ Card.YouCtrl,Emblem.YouCtrl | ValidTarget$ You | IsCombat$ False | ReplaceWith$ DamageReplace\n" +
	"SVar:DamageReplace:DB$ ReplaceEffect | VarName$ DamageAmount | VarValue$ ReplaceCount$DamageAmount/Plus.2\n" +
	"Oracle:x\n"

const emblemWardedSrc = "Name:Test Red Ward\nManaCost:1 W\nTypes:Creature Soldier\nPT:1/5\nK:Protection from red\nOracle:x\n"

// emblemUpkeep has seat 1 give seat 0 the Test Inferno emblem and resolves
// its trigger in seat 0's upkeep. It returns the engine, the maker and the
// log mark taken just before the trigger resolved.
func emblemUpkeep(t *testing.T, setup func(e *Engine)) (*Engine, state.ObjID, int) {
	t.Helper()
	e := layerEngine(t)
	maker := onBoard(t, e, 1, emblemMakerSrc)
	setup(e)
	f := e.G.Obj(maker).Face()
	var give *cards.SA
	for _, sa := range f.Abilities {
		if sa.API == "Effect" {
			give = sa
		}
	}
	if give == nil {
		t.Fatal("precondition: the maker has no Effect ability")
	}
	ctx := effects.NewCtx(maker, 1, effects.CtxInit{SVars: f.SVars})
	effects.Resolve(e, &ctx, give)
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].Controller != 0 {
		t.Fatalf("precondition: want one emblem trigger controlled by seat 0, got %+v", e.G.Delayed)
	}
	if e.G.Active != 0 {
		t.Fatalf("precondition: active seat = %d, want 0", e.G.Active)
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d objects, want the emblem trigger", len(e.G.Stack))
	}
	mark := len(e.L.Events)
	e.resolveTop()
	return e, maker, mark
}

func TestEmblemDamageIsNotFromASourceItsCreatorControls(t *testing.T) {
	t.Parallel()
	var life0, life1 int32
	var ward state.ObjID
	e, _, _ := emblemUpkeep(t, func(e *Engine) {
		onBoard(t, e, 1, emblemPitSrc)
		ward = onBoard(t, e, 0, emblemWardedSrc)
		life0, life1 = e.G.Players[0].Life, e.G.Players[1].Life
	})
	if got := life0 - e.G.Players[0].Life; got != 1 {
		t.Errorf("emblem owner lost %d life, want 1: the creator's +2 replacement does not see the emblem as a source it controls", got)
	}
	// CR 114.3: a colorless source with no lifelink.
	if got := e.G.Players[1].Life - life1; got != 0 {
		t.Errorf("creator's life changed by %d, want 0: the emblem has no lifelink", got)
	}
	if got := e.G.Obj(ward).Damage; got != 1 {
		t.Errorf("creature with protection from red took %d damage, want 1: the emblem is colorless", got)
	}
}

func TestEmblemDamageIsFromASourceItsOwnerControls(t *testing.T) {
	t.Parallel()
	var life0 int32
	e, maker, mark := emblemUpkeep(t, func(e *Engine) {
		onBoard(t, e, 0, emblemOwnPitSrc)
		life0 = e.G.Players[0].Life
	})
	if got := life0 - e.G.Players[0].Life; got != 3 {
		t.Errorf("emblem owner lost %d life, want 3: their own replacement applies to their emblem's damage", got)
	}
	seen := false
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind != events.DamageProvenance {
			continue
		}
		seen = true
		src := e.G.Obj(ev.Obj)
		if ev.Obj == maker || src == nil || src.Controller != 0 {
			t.Errorf("damage source = %d (%+v), want the emblem controlled by seat 0, not the maker %d", ev.Obj, src, maker)
		}
	}
	if !seen {
		t.Fatal("no damage provenance recorded")
	}
}

// TestEmblemDamageReplacementOrderKeepsTheEmblemAsSource: with two competing
// replacements the damage is parked for the CR 616.1 order choice, whose
// lifelink rider reads the emblem, not the lifelinking card that made it.
func TestEmblemDamageReplacementOrderKeepsTheEmblemAsSource(t *testing.T) {
	t.Parallel()
	var life0, life1 int32
	e, _, _ := emblemUpkeep(t, func(e *Engine) {
		onBoard(t, e, 0, emblemOwnPitSrc)
		onBoard(t, e, 0, emblemOwnPitSrc)
		life0, life1 = e.G.Players[0].Life, e.G.Players[1].Life
	})
	d := e.Pending()
	if d == nil || d.Kind != decision.KReplacement {
		t.Fatalf("precondition: want a replacement-order ask, got %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	if got := life0 - e.G.Players[0].Life; got != 5 {
		t.Errorf("emblem owner lost %d life, want 5 (1 plus 2 twice)", got)
	}
	if got := e.G.Players[1].Life - life1; got != 0 {
		t.Errorf("creator's life changed by %d, want 0: the emblem has no lifelink", got)
	}
}

// emblemMirrorSrc drains the controller of whatever source damages its owner,
// in place of that damage.
const emblemMirrorSrc = "Name:Test Mirror\nManaCost:2 W\nTypes:Enchantment\n" +
	"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ You | ReplaceWith$ Drain\n" +
	"SVar:Drain:DB$ LoseLife | Defined$ ReplacedSourceController | LifeAmount$ 4\n" +
	"Oracle:x\n"

// TestEmblemDamageReplacementBodyReadsTheEmblemsController: a replacement body
// naming the damage source's controller names the emblem's owner.
func TestEmblemDamageReplacementBodyReadsTheEmblemsController(t *testing.T) {
	t.Parallel()
	var life0, life1 int32
	e, _, _ := emblemUpkeep(t, func(e *Engine) {
		onBoard(t, e, 0, emblemMirrorSrc)
		life0, life1 = e.G.Players[0].Life, e.G.Players[1].Life
	})
	if got0, got1 := life0-e.G.Players[0].Life, life1-e.G.Players[1].Life; got0 != 4 || got1 != 0 {
		t.Errorf("life lost: owner %d, creator %d; want 4 and 0", got0, got1)
	}
}
