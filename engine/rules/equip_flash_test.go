package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// equipFlashOffered reports whether seat 0's pending priority decision offers an
// ability option on the equipment obj.
func equipFlashOffered(e *Engine, obj state.ObjID) bool {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == obj {
			return true
		}
	}
	return false
}

// equipBoard puts Bonesplitter (Equip {1}), a Grizzly Bears and the given
// extra permanents on seat 0's battlefield with one mana floating, then moves
// to the given step of the given player's turn with seat 0 holding priority.
func equipBoard(t *testing.T, extra string, turn state.PlayerID, step state.Step) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	moves := map[string]state.Zone{
		"Bonesplitter":  state.ZBattlefield,
		"Grizzly Bears": state.ZBattlefield,
	}
	if extra != "" {
		moves[extra] = state.ZBattlefield
	}
	e, _, ids := edrBoard(t, reg, 91, moves)
	addMana(t, e, 0, "R")
	if turn != 0 {
		e.emit(events.Event{Kind: events.TurnChange, Player: turn, Amount: 2})
	}
	e.emit(events.Event{Kind: events.StepChange, Step: step})
	e.priorityRound()
	edrSeatZeroPriority(t, e)
	return e, ids["Bonesplitter"]
}

// TestEquipAtInstantSpeed: CR 702.6 equip is sorcery-speed, except that Leonin
// Shikari's printed CastWithFlash static (ValidSA$ Activated.Equip) lets its
// controller activate equip any time they could cast an instant, and Forge
// Anew's copy only during its controller's turn (Condition$ PlayerTurn).
func TestEquipAtInstantSpeed(t *testing.T) {
	t.Parallel()
	// Precondition: at sorcery timing the equip is offered without any static.
	e, bs := equipBoard(t, "", 0, state.StepMain1)
	if !equipFlashOffered(e, bs) {
		t.Fatal("precondition: Equip is not offered in main phase 1 with an empty stack")
	}
	// Control: no static, combat -> equip is withheld.
	e, bs = equipBoard(t, "", 0, state.StepBeginCombat)
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || d.Player != 0 {
		t.Fatalf("control: seat 0 does not hold priority at beginning of combat: %+v", d)
	}
	if equipFlashOffered(e, bs) {
		t.Fatal("control: Equip offered at beginning of combat without a CastWithFlash static")
	}
	// Leonin Shikari: offered at the same step, on either player's turn.
	e, bs = equipBoard(t, "Leonin Shikari", 0, state.StepBeginCombat)
	if !equipFlashOffered(e, bs) {
		t.Error("Leonin Shikari: Equip withheld at beginning of combat")
	}
	e, bs = equipBoard(t, "Leonin Shikari", 1, state.StepBeginCombat)
	if !equipFlashOffered(e, bs) {
		t.Error("Leonin Shikari: Equip withheld on the opponent's turn")
	}
	// Forge Anew: only on its controller's turn.
	e, bs = equipBoard(t, "Forge Anew", 0, state.StepBeginCombat)
	if !equipFlashOffered(e, bs) {
		t.Error("Forge Anew: Equip withheld during its controller's turn")
	}
	e, bs = equipBoard(t, "Forge Anew", 1, state.StepBeginCombat)
	if equipFlashOffered(e, bs) {
		t.Error("Forge Anew: Equip offered on the opponent's turn (Condition$ PlayerTurn)")
	}
}
