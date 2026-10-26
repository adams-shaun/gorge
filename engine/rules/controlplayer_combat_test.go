package rules

// CR 723.2 limited-duration control: Secret of Bloodbending's unpaid mode
// ("You control target opponent during their next combat phase", Forge
// `Combat$ True`) is live only from that opponent's next beginning of combat
// step through its end of combat step and expires as that step ends; a
// whole-turn grant (Mindslaver) keeps the CR 723.1 next-turn interval.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const controlCombatAttacker = "Name:Combat Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// controlCombatBoard is the probe board: seat 0 holds the named corpus card,
// seat 1 has a non-sick creature on the battlefield (so its declare attackers
// step poses a real decision), and the game stands at seat 0's turn 3 main 1.
func controlCombatBoard(t *testing.T, name string) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, name)}, mountainDeck(t, 39)...),
			append([]*cards.Card{card(t, controlCombatAttacker)}, mountainDeck(t, 39)...),
		},
		Tokens: reg.Tokens,
	}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()
	id := findAndMoveToHand(t, e, 0, name)
	bear := moveByName(t, e, 1, "Combat Bear", state.ZBattlefield)
	e.G.Obj(bear).SummonSick = false
	e.pending = nil
	e.Advance()
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	return e, cfg, id
}

// liveAt walks seat 1's turn 4 to step and reports LiveController(1).
func liveAt(t *testing.T, e *Engine, step state.Step) bool {
	t.Helper()
	driveToStepAll(t, e, 4, 1, step)
	ctl, ok := e.G.LiveController(1)
	if ok && ctl != 0 {
		t.Fatalf("seat 1 live-controlled by %d, want 0", ctl)
	}
	return ok
}

func TestSecretOfBloodbendingControlsOnlyTheNextCombat(t *testing.T) {
	t.Parallel()
	e, cfg, id := controlCombatBoard(t, "Secret of Bloodbending")
	addMana(t, e, 0, "UUUU")
	submitChoices(t, e, castOptionIdx(e.Pending(), "cast", id))
	for i := 0; i < 40 && (len(e.G.Stack) > 0 || e.Pending() == nil || e.Pending().Kind != decision.KPriority || e.G.Obj(id).Zone == state.ZHand); i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KTarget:
			idx := indexOfPlayerOption(d, 1)
			if idx < 0 {
				t.Fatalf("target ask offered no opponent: %+v", d.Options)
			}
			submitChoices(t, e, idx)
		case decision.KPriority:
			submitPass(t, e)
		default:
			t.Fatalf("unexpected %s decision while casting: %+v", d.Kind, d.Options)
		}
	}
	if ctl, ok := e.G.ControlledBy[1]; !ok || ctl != 0 {
		t.Fatalf("after resolution ControlledBy = %+v, want seat 1 controlled by seat 0", e.G.ControlledBy)
	}
	if !e.G.ControlCombatOnly[1] {
		t.Fatalf("the unpaid mode's grant is not combat-only: %+v", e.G.ControlCombatOnly)
	}
	if e.G.ControlArmedTurn[1] != 3 {
		t.Fatalf("armed turn = %d, want 3", e.G.ControlArmedTurn[1])
	}

	if liveAt(t, e, state.StepUpkeep) {
		t.Fatal("combat-only control is live at seat 1's upkeep")
	}
	if liveAt(t, e, state.StepMain1) {
		t.Fatal("combat-only control is live at seat 1's main 1")
	}
	if !liveAt(t, e, state.StepBeginCombat) {
		t.Fatal("combat-only control is not live at seat 1's beginning of combat")
	}
	if !liveAt(t, e, state.StepDeclareAttackers) {
		t.Fatal("combat-only control is not live at seat 1's declare attackers")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("no attackers decision at seat 1's declare attackers (got %+v)", d)
	}
	if d.Player != 0 {
		t.Fatalf("seat 1's attackers decision posed to %d, want the controller 0", d.Player)
	}
	if !liveAt(t, e, state.StepEndCombat) {
		t.Fatal("combat-only control is not live at seat 1's end of combat")
	}
	if liveAt(t, e, state.StepMain2) {
		t.Fatal("combat-only control is live at seat 1's main 2")
	}
	if _, ok := e.G.ControlledBy[1]; ok {
		t.Fatalf("combat-only control outlived the combat: %+v", e.G.ControlledBy)
	}
	if len(e.G.ControlCombatOnly) != 0 {
		t.Fatalf("combat-only mark outlived the grant: %+v", e.G.ControlCombatOnly)
	}
	replayCheck(t, e, cfg)
}

func TestMindslaverControlStillLastsTheWholeTurn(t *testing.T) {
	t.Parallel()
	e, cfg, _ := controlCombatBoard(t, "Mindslaver")
	// The whole-turn grant exactly as effControlPlayer emits it for a
	// carrier without Combat$: no Counter.
	e.emit(events.Event{Kind: events.ControlPlayerChange, Player: 0,
		IDs: []state.ObjID{state.PlayerRef(1)}, Amount: 1})
	if ctl, ok := e.G.ControlledBy[1]; !ok || ctl != 0 || e.G.ControlCombatOnly[1] {
		t.Fatalf("precondition: whole-turn grant, got ControlledBy=%+v combatOnly=%+v",
			e.G.ControlledBy, e.G.ControlCombatOnly)
	}
	for _, s := range []state.Step{state.StepUpkeep, state.StepMain1, state.StepBeginCombat, state.StepMain2, state.StepEnd} {
		if !liveAt(t, e, s) {
			t.Fatalf("whole-turn control is not live at seat 1's %s", s)
		}
	}
	driveToStepAll(t, e, 5, 0, state.StepMain1)
	if len(e.G.ControlledBy) != 0 {
		t.Fatalf("whole-turn control outlived seat 1's turn: %+v", e.G.ControlledBy)
	}
	replayCheck(t, e, cfg)
}
