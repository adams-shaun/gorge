package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestDayNightChangeFoldAndPrevTurnSpells(t *testing.T) {
	g, l := twoPlayer(t)
	Emit(g, l, Event{Kind: DayNightChange, Amount: 1, Text: "day"})
	if g.DayNight != state.DayNightDay {
		t.Fatalf("amount 1: %v, want day", g.DayNight)
	}
	for _, bad := range []int32{0, 3, -1} {
		Emit(g, l, Event{Kind: DayNightChange, Amount: bad})
		if g.DayNight != state.DayNightDay {
			t.Fatalf("amount %d changed the designation to %v", bad, g.DayNight)
		}
	}
	Emit(g, l, Event{Kind: DayNightChange, Amount: 2, Text: "night"})
	if g.DayNight != state.DayNightNight || g.Clone().DayNight != state.DayNightNight {
		t.Fatalf("amount 2: %v (clone %v), want night", g.DayNight, g.Clone().DayNight)
	}

	active := g.Active
	other := 1 - active
	lib := g.Zone(state.ZLibrary, active)
	olib := g.Zone(state.ZLibrary, other)
	Emit(g, l, Event{Kind: PutOnStack, Obj: lib[0], Player: active, From: state.ZLibrary, To: state.ZStack})
	Emit(g, l, Event{Kind: PutOnStack, Obj: lib[1], Player: active, From: state.ZLibrary, To: state.ZStack})
	Emit(g, l, Event{Kind: PutOnStack, Obj: olib[0], Player: other, From: state.ZLibrary, To: state.ZStack})
	if g.Players[active].SpellsCastThisTurn != 2 {
		t.Fatalf("precondition: active seat cast %d, want 2", g.Players[active].SpellsCastThisTurn)
	}
	Emit(g, l, Event{Kind: TurnChange, Player: other, Amount: g.Turn + 1})
	if g.PrevTurnSpells != 2 {
		t.Fatalf("PrevTurnSpells = %d, want 2", g.PrevTurnSpells)
	}
	if got := g.Players[active].SpellsCastThisTurn; got != 0 {
		t.Fatalf("outgoing seat's tally = %d after the turn change, want 0", got)
	}
}
