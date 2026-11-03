package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	dilemmaCharm = "Name:Two Roads\nManaCost:U\nTypes:Sorcery\n" +
		"A:SP$ Charm | CharmNum$ 2 | MinCharmNum$ 0 | Choices$ RoadA,RoadB\n" +
		"SVar:RoadA:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ Gain one.\n" +
		"SVar:RoadB:DB$ GainLife | Defined$ You | LifeAmount$ 2 | SpellDescription$ Gain two.\nOracle:x\n"
	dilemmaWatcher = "Name:Deliberator\nManaCost:1 U\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ FacesDilemma | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ DrawOne | TriggerDescription$ x\n" +
		"SVar:DrawOne:DB$ Draw | NumCards$ 1 | Defined$ TriggeredPlayer\nOracle:x\n"
	dilemmaTally = "Name:Tally Keeper\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n" +
		"T:Mode$ Phase | Phase$ End of Turn | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ GainTally | TriggerDescription$ x\n" +
		"SVar:GainTally:DB$ GainLife | LifeAmount$ Faced | Defined$ You\nSVar:Faced:Count$YouFaceDilemmaThisTurn\nOracle:x\n"
)

// castDilemmaCharm casts the Charm from hand choosing the given mode indices.
func castDilemmaCharm(t *testing.T, e *Engine, spell state.ObjID, modes ...int) {
	t.Helper()
	submitChoices(t, e, miscCastOption(t, e, spell))
	if d := e.Pending(); d == nil || !strings.Contains(d.Prompt, "mode") {
		t.Fatalf("no mode ask: %+v", d)
	}
	submitChoices(t, e, modes...)
}

// TestChoosingModesFacesOneDilemma: the Star Trek release notes. Choosing two
// modes is one dilemma, and the trigger resolves before the spell that caused it.
func TestChoosingModesFacesOneDilemma(t *testing.T) {
	t.Parallel()
	e, cfg, spell := newFixtureDeck(t, 951, dilemmaCharm, dilemmaWatcher)
	addMana(t, e, 0, "U")
	putCreature(t, e, 0, dilemmaWatcher)
	e.Advance()
	hand := len(e.G.Zone(state.ZHand, 0))
	castDilemmaCharm(t, e, spell, 0, 1)
	if got := e.G.Players[0].DilemmasFacedThisTurn; got != 1 {
		t.Fatalf("dilemmas faced = %d, want 1 for two modes", got)
	}
	if len(e.G.Stack) != 2 || e.G.Stack[0] != spell {
		t.Fatalf("stack %v, want the trigger above the spell %d", e.G.Stack, spell)
	}
	resolveStack(t, e)
	if got := len(e.G.Zone(state.ZHand, 0)); got != hand {
		t.Fatalf("hand %d, want %d (cast one, drew one)", got, hand)
	}
	replayCheck(t, e, cfg)
}

// TestChoosingNoModesFacesNoDilemma: "up to" with nothing chosen.
func TestChoosingNoModesFacesNoDilemma(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 952, dilemmaCharm, dilemmaWatcher)
	addMana(t, e, 0, "U")
	putCreature(t, e, 0, dilemmaWatcher)
	e.Advance()
	castDilemmaCharm(t, e, spell)
	if got := e.G.Players[0].DilemmasFacedThisTurn; got != 0 {
		t.Fatalf("dilemmas faced = %d, want 0", got)
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack %v, want the spell alone", e.G.Stack)
	}
}

// TestDilemmaCountReadsAndClearsWithTheTurn: Count$YouFaceDilemmaThisTurn reads
// the turn's choices and starts over the next turn.
func TestDilemmaCountReadsAndClearsWithTheTurn(t *testing.T) {
	t.Parallel()
	e, cfg, spell := newFixtureDeck(t, 953, dilemmaCharm, dilemmaTally)
	addMana(t, e, 0, "U")
	putCreature(t, e, 0, dilemmaTally)
	e.Advance()
	castDilemmaCharm(t, e, spell, 1)
	resolveStack(t, e)
	life := e.G.Players[0].Life
	driveToStep(t, e, e.G.Turn, 0, state.StepEnd)
	resolveStack(t, e)
	if got := e.G.Players[0].Life; got != life+1 {
		t.Fatalf("life %d at the end step, want %d for one dilemma", got, life+1)
	}
	passUntilTurn(t, e, e.G.Turn+1)
	if got := e.G.Players[0].DilemmasFacedThisTurn; got != 0 {
		t.Fatalf("dilemmas after the turn change = %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestReversedCastUndoesItsDilemma: CR 733.1 takes the choice back with the cast.
func TestReversedCastUndoesItsDilemma(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 954, dilemmaCharm, dilemmaWatcher)
	addMana(t, e, 0, "U")
	putCreature(t, e, 0, dilemmaWatcher)
	e.Advance()
	submitChoices(t, e, miscCastOption(t, e, spell))
	e.emit(events.Event{Kind: events.ManaClear, Player: 0})
	submitChoices(t, e, 0)
	if z := e.G.Obj(spell).Zone; z != state.ZHand {
		t.Fatalf("reversed spell is in %s, want hand", z)
	}
	if got := e.G.Players[0].DilemmasFacedThisTurn; got != 0 {
		t.Fatalf("dilemmas after the reversal = %d, want 0", got)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack %v, want the dilemma trigger dropped with the proposal", e.G.Stack)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen && ev.Counter == events.DilemmaReversed {
			return
		}
	}
	t.Fatal("no reversal marker in the log")
}

// TestGenericChoiceIsNotADilemma: GenericChoice asks through the same marker but
// is not modal.
func TestGenericChoiceIsNotADilemma(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 955, dilemmaCharm, dilemmaWatcher)
	generic := card(t, "Name:Pick\nManaCost:U\nTypes:Sorcery\nA:SP$ GenericChoice | Choices$ A,B\nSVar:A:DB$ GainLife | LifeAmount$ 1\nSVar:B:DB$ GainLife | LifeAmount$ 2\nOracle:x\n")
	if got := e.dilemmaCounter(generic.Faces[0].SpellAbility(), 1); got != "" {
		t.Fatalf("a GenericChoice marked %q", got)
	}
}

// TestNoReaderLeavesTheMarkerAlone: a game with no card that reads dilemmas logs
// the ModeChosen a recorded game has, so it replays unchanged.
func TestNoReaderLeavesTheMarkerAlone(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 956, dilemmaCharm)
	addMana(t, e, 0, "U")
	e.Advance()
	castDilemmaCharm(t, e, spell, 0)
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen && ev.Counter != "" {
			t.Fatalf("marked with no reader: %+v", ev)
		}
	}
}

// TestModalCastDuringAResolutionFacesOneDilemma: a free cast's modes are answered
// inside the resolution that casts it, and the choice is still one dilemma.
func TestModalCastDuringAResolutionFacesOneDilemma(t *testing.T) {
	t.Parallel()
	const caster = "Name:Free Caster\nManaCost:U\nTypes:Sorcery\n" +
		"A:SP$ Play | Valid$ Card.namedTwo Roads | ValidZone$ Hand | WithoutManaCost$ True | Optional$ True | Controller$ You\nOracle:x\n"
	e, _, spell := newFixtureDeck(t, 952, caster, dilemmaCharm, dilemmaWatcher)
	addMana(t, e, 0, "U")
	putCreature(t, e, 0, dilemmaWatcher)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if e.G.Obj(id).Face().Name == "Two Roads" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
			break
		}
	}
	e.Advance()
	submitChoices(t, e, miscCastOption(t, e, spell))
	castFirst(t, e, "pass")
	castFirst(t, e, "pass")
	submitChoices(t, e, 0) // play it
	submitChoices(t, e, 0) // one mode
	if got := e.G.Players[0].DilemmasFacedThisTurn; got != 1 {
		t.Fatalf("dilemmas faced = %d, want 1", got)
	}
}
