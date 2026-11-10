package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestFoldDelayedRegisterGrantor pins the "|GB=<id>" Text spelling of a
// registration made by a GRANTED ability (state.DelayedTrigger.Grantor): it
// rides after |TT= and folds to the grantor while the neighbour slots stay
// intact. The same Text without it (an already-logged registration) folds to
// Grantor zero exactly as before.
func TestFoldDelayedRegisterGrantor(t *testing.T) {
	fold := func(text string) state.DelayedTrigger {
		t.Helper()
		g := state.NewGame([]string{"Ann", "Bob"})
		card, _ := cards.ParseBytes("c.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
		id := g.AddObject(card, 0).ID
		grantor := g.AddObject(card, 0).ID
		g.SetZone(state.ZBattlefield, 0, []state.ObjID{id, grantor})
		g.Obj(id).Zone = state.ZBattlefield
		g.Obj(grantor).Zone = state.ZBattlefield
		Apply(g, Event{Kind: DelayedRegister, Obj: id, Player: 0, Step: state.StepEndCombat,
			Counter: "Body", IDs: []state.ObjID{id}, Text: text})
		if len(g.Delayed) != 1 {
			t.Fatalf("Text %q folded %d registrations, want 1", text, len(g.Delayed))
		}
		return g.Delayed[0]
	}
	with := fold("SpellCast:Mode$ SpellCast | ValidCard$ Instant,Sorcery|TT=3|GB=2")
	if with.Grantor != 2 {
		t.Errorf("with |GB=: Grantor = %d, want 2", with.Grantor)
	}
	if with.MaxTurn != 3 || with.EventMode != "SpellCast" || with.Execute != "Body" {
		t.Errorf("with |GB=: MaxTurn=%d EventMode=%q Execute=%q, want 3, SpellCast, Body",
			with.MaxTurn, with.EventMode, with.Execute)
	}
	without := fold("SpellCast:Mode$ SpellCast | ValidCard$ Instant,Sorcery|TT=3")
	if without.Grantor != 0 {
		t.Errorf("without |GB=: Grantor = %d, want 0 (old log)", without.Grantor)
	}
	if without.MaxTurn != 3 || without.EventMode != "SpellCast" {
		t.Errorf("without |GB=: MaxTurn=%d EventMode=%q, want 3, SpellCast", without.MaxTurn, without.EventMode)
	}
}
