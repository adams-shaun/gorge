package events

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func bearCard() *cards.Card {
	c, _ := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	c.Link()
	return c
}

func twoPlayer(t *testing.T) (*state.Game, *Log) {
	t.Helper()
	g := state.NewGame([]string{"a", "b"})
	bear := bearCard()
	for p := state.PlayerID(0); p < 2; p++ {
		var lib []state.ObjID
		for i := 0; i < 5; i++ {
			lib = append(lib, g.AddObject(bear, p).ID)
		}
		g.SetZone(state.ZLibrary, p, lib)
	}
	return g, NewLog(1)
}

// Every object must be in exactly one zone at all times. This is the invariant
// that a naive "move plus push" pair of events silently breaks.
func zoneCount(g *state.Game, id state.ObjID) int {
	n := 0
	for p := state.PlayerID(0); p < state.PlayerID(len(g.Players)); p++ {
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			for _, x := range g.Zone(z, p) {
				if x == id {
					n++
				}
			}
		}
	}
	for _, x := range g.Stack {
		if x == id {
			n++
		}
	}
	return n
}

func TestMoveKeepsExactlyOneZone(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	for _, step := range []struct{ from, to state.Zone }{
		{state.ZLibrary, state.ZHand},
		{state.ZHand, state.ZStack},
		{state.ZStack, state.ZBattlefield},
		{state.ZBattlefield, state.ZGraveyard},
		{state.ZGraveyard, state.ZExile},
	} {
		Emit(g, l, Event{Kind: MoveZone, Obj: id, From: step.from, To: step.to})
		if got := zoneCount(g, id); got != 1 {
			t.Fatalf("after %s->%s object is in %d zones", step.from, step.to, got)
		}
		if g.Obj(id).Zone != step.to {
			t.Fatalf("Obj.Zone = %s, want %s", g.Obj(id).Zone, step.to)
		}
	}
	if len(g.Zone(state.ZLibrary, 0)) != 4 {
		t.Fatalf("library = %d, want 4", len(g.Zone(state.ZLibrary, 0)))
	}
}

func TestPutOnStackDoesNotDoublePush(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
	Emit(g, l, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	if len(g.Stack) != 1 {
		t.Fatalf("stack = %v, want one entry", g.Stack)
	}
	if zoneCount(g, id) != 1 {
		t.Fatal("PutOnStack duplicated the object")
	}
	// Resolve is a marker: the object leaves via its own move event.
	Emit(g, l, Event{Kind: Resolve, Obj: id})
	if len(g.Stack) != 1 {
		t.Fatal("Resolve must not pop the stack")
	}
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZGraveyard})
	if len(g.Stack) != 0 || zoneCount(g, id) != 1 {
		t.Fatal("resolution move did not clear the stack cleanly")
	}
}

func TestScalarEventsFold(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 0)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})

	Emit(g, l, Event{Kind: LifeChange, Player: 1, Amount: -3})
	if g.Players[1].Life != 17 {
		t.Errorf("life = %d", g.Players[1].Life)
	}
	Emit(g, l, Event{Kind: Damage, Player: 1, Amount: 2})
	if g.Players[1].Life != 15 {
		t.Errorf("damage to a player must reduce life: %d", g.Players[1].Life)
	}
	Emit(g, l, Event{Kind: Damage, Obj: id, Amount: 1})
	if g.Obj(id).Damage != 1 {
		t.Errorf("object damage = %d", g.Obj(id).Damage)
	}
	Emit(g, l, Event{Kind: Tap, Obj: id})
	if !g.Obj(id).Tapped {
		t.Error("tap did not apply")
	}
	Emit(g, l, Event{Kind: Untap, Obj: id})
	if g.Obj(id).Tapped {
		t.Error("untap did not apply")
	}
	Emit(g, l, Event{Kind: CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	Emit(g, l, Event{Kind: CounterChange, Obj: id, Counter: "P1P1", Amount: -5})
	if got := g.Obj(id).Counter("P1P1"); got != 0 {
		t.Errorf("counters clamp at zero, got %d", got)
	}
	Emit(g, l, Event{Kind: ManaAdd, Player: 0, Counter: "R", Amount: 2})
	if g.Players[0].Pool[state.MR] != 2 {
		t.Errorf("pool = %v", g.Players[0].Pool)
	}
	Emit(g, l, Event{Kind: ManaClear, Player: 0})
	if g.Players[0].Pool.Total() != 0 {
		t.Error("mana clear did not empty the pool")
	}
}

func TestTurnChangeResetsPerTurnState(t *testing.T) {
	g, l := twoPlayer(t)
	id := g.Zone(state.ZLibrary, 1)[0]
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	if !g.Obj(id).SummonSick {
		t.Fatal("entering the battlefield must set summoning sickness")
	}
	g.Players[1].LandsPlayed = 1
	Emit(g, l, Event{Kind: TurnChange, Player: 1, Amount: 4})
	if g.Turn != 4 || g.Active != 1 {
		t.Fatalf("turn = %d active = %d", g.Turn, g.Active)
	}
	if g.Players[1].LandsPlayed != 0 {
		t.Error("land drop not reset")
	}
	if g.Obj(id).SummonSick {
		t.Error("summoning sickness not cleared for the active player")
	}
}

func TestShuffleReplacesLibraryOrder(t *testing.T) {
	g, l := twoPlayer(t)
	want := []state.ObjID{5, 4, 3, 2, 1}
	Emit(g, l, Event{Kind: Shuffle, Player: 0, IDs: want, Secret: true})
	got := g.Zone(state.ZLibrary, 0)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("library = %v, want %v", got, want)
		}
	}
	// The log must own its copy: mutating the caller's slice cannot rewrite
	// history.
	want[0] = 99
	if g.Zone(state.ZLibrary, 0)[0] == 99 {
		t.Fatal("Shuffle aliased the caller's slice")
	}
}
