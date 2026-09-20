package rules

// The api:GenericChoice primitive (task agent-20260918T202223Z-eb7aab2a):
// Forge's free-form modal choice ("create a Food token OR a Treasure token")
// is a modal machinery carrier like Charm — a Choices$ list naming SVar
// ability bodies — and since it registered onto the same handler
// (effects/misc.go: GenericChoice -> effCharm) both of its paths resolve:
// a trigger body poses the CR 603.3c placement KModes ask (askTriggerModes,
// rules/trigger_queue.go) and a DB$ reached mid-resolution poses effCharm's
// own KModes ask.
//
// The Tireless Provisioner pins inline the REAL tireless_provisioner.txt
// script (never a .cards file — the licensing rule) so the corpus carrier's
// exact parameter spellings are what the engine defends. The token scripts
// are authored here under the corpus's TokenScript$ keys so the inline
// carrier's references resolve.
//
// Out of scope, recorded in the ticket's ledger: effCharm asks the resolving
// controller and ignores Defined$ (the same narrowing Charm already has), and
// an SP$ GenericChoice spell announces its modes mid-resolution rather than at
// CR 601.2b cast time (castModeAsk is Charm-only) — pinned as the stand-in it
// is by TestGenericChoiceSpellAsksMidResolution.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// provisionerScript is tireless_provisioner.txt verbatim except the Oracle
// line's parenthetical token reminder and the DeckHas rider (display/rider
// text the engine never reads).
const provisionerScript = "Name:Tireless Provisioner\nManaCost:2 G\nTypes:Creature Elf Scout\nPT:3/2\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Land.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigTokenChoice | TriggerDescription$ Landfall — Whenever a land you control enters, create a Food token or a Treasure token.\n" +
	"SVar:TrigTokenChoice:DB$ GenericChoice | Defined$ You | Choices$ Food,Treasure | AILogic$ FoodOrTreasure | SpellDescription$ Whenever a land you control enters, create a Food token or a Treasure token.\n" +
	"SVar:Food:DB$ Token | TokenScript$ c_a_food_sac | TokenOwner$ You | SpellDescription$ Food\n" +
	"SVar:Treasure:DB$ Token | TokenScript$ c_a_treasure_sac | TokenOwner$ You | SpellDescription$ Treasure\n" +
	"Oracle:x\n"

const provForestScript = "Name:Forest\nManaCost:no cost\nTypes:Land Forest\nOracle:x\n"

// Authored token fixtures under the corpus's own TokenScript$ keys (the GPL
// .cards/tokenscripts text is never copied; these are minimal stand-ins whose
// names match the corpus faces).
const foodTokenScript = "Name:Food Token\nManaCost:no cost\nTypes:Artifact Food\nOracle:x\n"
const treasureTokenScript = "Name:Treasure Token\nManaCost:no cost\nTypes:Artifact Treasure\nOracle:x\n"

// provisionerAtModes drives a battlefield Tireless Provisioner through a land
// entering under seat 0's control to its Landfall trigger's placement modes
// ask and returns the engine with that decision pending.
func provisionerAtModes(t *testing.T, seed uint64) (*Engine, Config) {
	t.Helper()
	e, cfg, _ := newFixtureDeck(t, seed, provisionerScript, provForestScript)
	e.G.Tokens["c_a_food_sac"] = card(t, foodTokenScript)
	e.G.Tokens["c_a_treasure_sac"] = card(t, treasureTokenScript)
	putCreature(t, e, 0, provisionerScript)
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Fatalf("after the provisioner enters, pending = %+v, want priority (its own entry must not fire Landfall)", d)
	}
	gateMoveFromLibrary(t, e, "Forest", state.ZBattlefield)
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %v, want exactly the Landfall trigger", e.G.Stack)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "modes" {
		t.Fatalf("pending = %+v, want the placement KModes ask", d)
	}
	if len(d.Options) != 2 {
		t.Fatalf("mode options = %d, want the two Choices$ candidates", len(d.Options))
	}
	labels := map[string]bool{}
	for _, o := range d.Options {
		labels[o.Label] = true
	}
	if !labels["Food"] || !labels["Treasure"] {
		t.Fatalf("modal labels %+v, want exactly Food and Treasure (the SVars' SpellDescription$)", d.Options)
	}
	return e, cfg
}

func TestTirelessProvisionerLandfallCreatesChosenFood(t *testing.T) {
	e, cfg := provisionerAtModes(t, 6303)
	submitChoices(t, e, 0) // Food
	passUntilStackEmpty(t, e, 20)
	if tokID := findByName(e, "Food Token", 0); tokID == 0 {
		t.Fatal("the Food answer created no Food token")
	} else if o := e.G.Obj(tokID); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("Food token zone/controller = %s/%d, want Battlefield/0", o.Zone, o.Controller)
	}
	if findByName(e, "Treasure Token", 0) != 0 {
		t.Fatal("the Food answer created a Treasure token anyway")
	}
	replayCheck(t, e, cfg)
}

func TestTirelessProvisionerLandfallCreatesChosenTreasure(t *testing.T) {
	e, cfg := provisionerAtModes(t, 6304)
	submitChoices(t, e, 1) // Treasure
	passUntilStackEmpty(t, e, 20)
	if tokID := findByName(e, "Treasure Token", 0); tokID == 0 {
		t.Fatal("the Treasure answer created no Treasure token")
	} else if o := e.G.Obj(tokID); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("Treasure token zone/controller = %s/%d, want Battlefield/0", o.Zone, o.Controller)
	}
	if findByName(e, "Food Token", 0) != 0 {
		t.Fatal("the Treasure answer created a Food token anyway")
	}
	replayCheck(t, e, cfg)
}

// TestGenericChoiceSpellAsksMidResolution pins the second path: a spell
// (SP$) carrier's GenericChoice poses effCharm's own KModes ask at
// resolution — the CR 601.2b cast-announcement timing (castModeAsk) is
// Charm-only and stays so (the recorded stand-in), but the resolution itself
// is real.
func TestGenericChoiceSpellAsksMidResolution(t *testing.T) {
	const spellSrc = "Name:Rite of Flux\nManaCost:U\nTypes:Instant\n" +
		"A:SP$ GenericChoice | Choices$ LoseIt,GainIt | SpellDescription$ Choose.\n" +
		"SVar:LoseIt:DB$ LoseLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ LoseIt\n" +
		"SVar:GainIt:DB$ GainLife | LifeAmount$ 2 | SpellDescription$ GainIt\n" +
		"Oracle:x\n"
	e, cfg, sp := newFixtureDeck(t, 6305, spellSrc)
	addMana(t, e, 0, "U")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("pending = %+v, want priority", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == sp {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the spell: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit cast: %v", err)
	}
	d = passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "modes" {
		t.Fatalf("pending after the cast = %+v, want the mid-resolution KModes ask", d)
	}
	if got := len(d.Options); got != 2 {
		t.Fatalf("mode options = %d, want 2", got)
	}
	life0 := e.G.Players[0].Life
	submitChoices(t, e, 1) // GainIt
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != life0+2 {
		t.Fatalf("player 0 life = %d, want %d (GainIt resolved)", got, life0+2)
	}
	replayCheck(t, e, cfg)
}
