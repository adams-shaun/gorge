package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDainEnduringStoryAttackTax(t *testing.T) {
	e, attacker := attackPropSeat(t, "Dáin, Lord of the Iron Hills", 0)
	// Dáin himself is legendary; these two artifacts complete the three.
	mox := "Name:Story Artifact\nManaCost:0\nTypes:Artifact\nOracle:x\n"
	onBoard(t, e, 0, mox)
	onBoard(t, e, 0, mox)
	// Move one artifact onto the battlefield through the event fold to run the
	// same post-fold grant check used by live game entry.
	artifact := e.G.Zone(state.ZBattlefield, 0)[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifact, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: artifact, From: state.ZGraveyard, To: state.ZBattlefield})
	if !e.G.Players[0].EnduringStory {
		t.Fatal("three qualifying permanents including real Storied Dáin did not grant enduring story")
	}
	if e.G.Obj(attacker) == nil || e.G.Obj(attacker).Zone != state.ZBattlefield {
		t.Fatal("attacker fixture is not on battlefield")
	}
	// The designation is permanent even after all qualifying permanents leave.
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZBattlefield, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	}
	if !e.G.Players[0].EnduringStory {
		t.Fatal("enduring story cleared after permanents left")
	}
}
