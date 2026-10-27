package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func suffixSeats(ts []state.Target) []state.PlayerID {
	var out []state.PlayerID
	for _, t := range ts {
		if !t.IsPlayer {
			return nil
		}
		out = append(out, t.Player)
	}
	return out
}

func suffixDefined(h *fakeHost, c *Ctx, spec string) []state.Target {
	return Defined(h, c, &cards.SA{Params: map[string]string{"Defined": spec}})
}

// TestTriggerReplacedPlayerSuffixes pins the Opponent / Owner / Controller
// spellings on a three-seat game whose triggering (or replaced) object is
// OWNED by seat 2 and CONTROLLED by seat 1, with the source under seat 0, so
// the source-controller fallback, the controller and the owner all differ.
func TestTriggerReplacedPlayerSuffixes(t *testing.T) {
	h := newHost(t, 3)
	card := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	src := h.g.AddObject(card, 0)
	b := h.g.AddObject(card, 2)
	b.Controller = 1
	if b.Owner != 2 || b.Controller != 1 || src.Controller != 0 {
		t.Fatalf("fixture precondition: owner=%d controller=%d source controller=%d", b.Owner, b.Controller, src.Controller)
	}
	eq := func(name string, got []state.Target, want ...state.PlayerID) {
		t.Helper()
		g := suffixSeats(got)
		if len(g) != len(want) {
			t.Fatalf("%s = %v, want %v", name, got, want)
		}
		for i := range want {
			if g[i] != want[i] {
				t.Fatalf("%s = %v, want %v", name, got, want)
			}
		}
	}

	trig := &Ctx{Source: src.ID, Controller: 0, TriggerContext: TriggerContext{TriggerCard: b.ID}}
	eq("TriggeredCardController", suffixDefined(h, trig, "TriggeredCardController"), 1)
	eq("TriggeredCardOpponent", suffixDefined(h, trig, "TriggeredCardOpponent"), 0, 2)
	eq("TriggeredAttackerOwner", suffixDefined(h, trig, "TriggeredAttackerOwner"), 2)

	repl := &Ctx{Source: src.ID, Controller: 0}
	repl.Repl.Replaced = b.ID
	eq("ReplacedCardController", suffixDefined(h, repl, "ReplacedCardController"), 1)

	// The LKI form: seat 1 controlled the object when the replacement
	// matched; it is now controlled by seat 2 (as after the body's move).
	b.Controller = 2
	repl.Repl.ReplacedController, repl.Repl.HasReplacedController = 1, true
	eq("ReplacedCardLKIController", suffixDefined(h, repl, "ReplacedCardLKIController"), 1)
	repl.Repl.HasReplacedController = false
	eq("ReplacedCardLKIController uncaptured", suffixDefined(h, repl, "ReplacedCardLKIController"), 2)

	// Absent binding: nobody, never the source's controller.
	empty := &Ctx{Source: src.ID, Controller: 0}
	for _, spec := range []string{"TriggeredCardOpponent", "TriggeredAttackerOwner", "ReplacedCardController", "ReplacedCardLKIController"} {
		if got := suffixDefined(h, empty, spec); len(got) != 0 {
			t.Errorf("%s with no binding = %v, want nobody", spec, got)
		}
	}
}
