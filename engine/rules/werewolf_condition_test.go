package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// werewolfGame is a two-seat game whose seat-0 deck holds the corpus Gatstaf
// Shepherd and an authored one-mana instant; the Shepherd is on seat 0's
// battlefield in turn 1's first main phase.
func werewolfGame(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	deck := []*cards.Card{searchCorpusCard(t, reg, "Gatstaf Shepherd"), card(t, dayNightPingSrc)}
	deck = append(deck, mountainDeck(t, 38)...)
	cfg := seatZeroStart(Config{Seed: 9411, Names: []string{"wolf", "other"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := searchMoveByName(t, e, "Gatstaf Shepherd", state.ZBattlefield)
	return e, cfg, id
}

func werewolfFlips(e *Engine, id state.ObjID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.FlipFace && ev.Obj == id {
			n++
		}
	}
	return n
}

// TestWerewolfTransformConditionGatesUpkeep is CR 603.4: Gatstaf Shepherd's
// "at the beginning of each upkeep, if no spells were cast last turn,
// transform" is an intervening-if (WerewolfTransformCondition$). A spell cast
// in turn 1 keeps it from transforming at turn 2's upkeep; with no spell cast
// it transforms.
func TestWerewolfTransformConditionGatesUpkeep(t *testing.T) {
	t.Run("spell cast last turn", func(t *testing.T) {
		e, cfg, id := werewolfGame(t)
		castVanilla(t, e, "Ping", "R")
		driveToStep(t, e, 2, 1, state.StepDraw)
		if n := werewolfFlips(e, id); n != 0 || e.G.Obj(id).FaceIdx != 0 {
			t.Fatalf("after a spell last turn: %d flips, face %d; want no transform", n, e.G.Obj(id).FaceIdx)
		}
		replayCheck(t, e, cfg)
	})
	t.Run("no spell last turn", func(t *testing.T) {
		e, cfg, id := werewolfGame(t)
		driveToStep(t, e, 2, 1, state.StepDraw)
		if n := werewolfFlips(e, id); n != 1 || e.G.Obj(id).FaceIdx != 1 {
			t.Fatalf("with no spell last turn: %d flips, face %d; want one transform", n, e.G.Obj(id).FaceIdx)
		}
		replayCheck(t, e, cfg)
	})
}
