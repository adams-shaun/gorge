package cardcov

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBranchKeysFromEvents credits a spell's X, alternative cost and response
// branches from a hand-built event log: obj 7 is cast with X 0 and kicked, obj
// 9 goes on the stack above it, then obj 7 resolves.
func TestBranchKeysFromEvents(t *testing.T) {
	t.Parallel()
	kicked := events.FlagsString(state.FlagKicked)
	evs := []events.Event{
		{Kind: events.PutOnStack, Obj: 7, To: state.ZStack},
		{Kind: events.CastInfo, Obj: 7, Amount: 0, Counter: kicked},
		{Kind: events.PutOnStack, Obj: 9, To: state.ZStack},
		{Kind: events.Resolve, Obj: 7},
	}
	owner := func(id state.ObjID) string {
		if id == 7 {
			return "Card A"
		}
		return ""
	}
	inv := func(name string) BranchInv {
		if name != "Card A" {
			return BranchInv{}
		}
		return BranchInv{HasX: true, AltFlags: []string{kicked}, SpellFaces: []int{0}}
	}
	got := Branches(evs, []decision.Intent(nil), 2, owner, inv)
	for _, k := range []string{"cast/alt:" + kicked, "f0/s", "f0/s/x0", "ix:resp"} {
		if got["Card A"][k] != 1 {
			t.Errorf("Card A key %q = %d, want 1 (got %v)", k, got["Card A"][k], got["Card A"])
		}
	}
	for _, k := range []string{"f0/s/x+", "cast/alt:none", "ix:mp"} {
		if got["Card A"][k] != 0 {
			t.Errorf("Card A key %q credited (%d), want none", k, got["Card A"][k])
		}
	}
	if len(got) != 1 {
		t.Errorf("only Card A may be credited, got %v", got)
	}
}

// TestBranchKeysOptionalAndMultiplayer credits an accepted optional trigger
// through the intent that answered its DecisionAsk, an unkicked cast of a card
// with a kicker, X above zero, and ix:mp only in a 3-seat game.
func TestBranchKeysOptionalAndMultiplayer(t *testing.T) {
	t.Parallel()
	evs := []events.Event{
		{Kind: events.PutOnStack, Obj: 5, To: state.ZStack},
		{Kind: events.CastInfo, Obj: 5, Amount: 3},
		{Kind: events.Resolve, Obj: 5},
		{Kind: events.DecisionAsk, Text: string(decision.KTriggerOptional)},
		{Kind: events.DecisionMade, Text: "trigger_optional:[0]"},
		{Kind: events.TriggerPush, Obj: 5, Amount: 1},
	}
	intents := []decision.Intent{{Seq: 3, Choices: []int{0}}}
	owner := func(id state.ObjID) string {
		if id == 5 {
			return "Card B"
		}
		return ""
	}
	inv := func(string) BranchInv {
		return BranchInv{HasX: true, AltFlags: []string{"kicked"}, OptTriggers: []string{"f0/t1"}, SpellFaces: []int{0}}
	}
	for seats, wantMP := range map[int]int{2: 0, 3: 1} {
		got := Branches(evs, intents, seats, owner, inv)["Card B"]
		for k, want := range map[string]int{"f0/s/x+": 1, "f0/s/x0": 0, "cast/alt:none": 1, "cast/alt:kicked": 0, "f0/t1/opt:y": 1, "f0/t1/opt:n": 0, "ix:mp": wantMP} {
			if got[k] != want {
				t.Errorf("%d seats: key %q = %d, want %d (got %v)", seats, k, got[k], want, got)
			}
		}
	}
}
