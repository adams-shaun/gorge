package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func definedRevealShuffles(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			n++
		}
	}
	return n
}

// A peeked top card that is revealed and put into hand is a Defined$ fetch,
// not a library search: it owes no shuffle (Forge's `defined` flag).
func TestDefinedLibraryRevealToHandDoesNotShuffle(t *testing.T) {
	for _, tc := range []struct{ card, svar string }{
		{"Traveling Botanist", "DBToHand"},
		{"Gathering Stone", "DBToHand"},
		{"Bucolic Ranch", "DBChangeZone"},
	} {
		t.Run(tc.card, func(t *testing.T) {
			_, ability, vars := corpusRiderSA(t, tc.card, tc.svar)
			if ability.API != "ChangeZone" || ability.Params["Defined"] != "Remembered" ||
				ability.Params["Origin"] != "Library" || ability.Params["Destination"] != "Hand" {
				t.Fatalf("precondition: %s %s params %+v", tc.card, tc.svar, ability.Params)
			}
			h := newHost(t, 2)
			source := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
			revealed := h.g.AddObject(mkCard(t, "Name:Revealed\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
			under := h.g.AddObject(mkCard(t, "Name:Under\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
			h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{source.ID})
			h.g.SetZone(state.ZLibrary, 0, []state.ObjID{revealed.ID, under.ID})
			h.g.Obj(revealed.ID).Zone = state.ZLibrary
			h.g.Obj(under.ID).Zone = state.ZLibrary
			if h.g.Obj(revealed.ID).Zone != state.ZLibrary || revealed.ID == under.ID {
				t.Fatal("precondition: revealed card is the distinct top card of the library")
			}
			Resolve(h, &Ctx{Source: source.ID, Controller: 0, SVars: vars,
				Remembered: []state.Target{{Obj: revealed.ID}}}, ability)
			if got := h.g.Obj(revealed.ID).Zone; got != state.ZHand {
				t.Fatalf("revealed card zone = %s, want hand", got)
			}
			if n := definedRevealShuffles(h); n != 0 {
				t.Fatalf("%s %s emitted %d Shuffle events, want none: %+v", tc.card, tc.svar, n, h.log)
			}
		})
	}

	// Positive controls: the shuffle owed by a deferred search, a NoLooking$
	// placement leg and an explicit Shuffle$ True is kept.
	for _, tc := range []struct {
		name     string
		script   string
		deferred bool
	}{
		{"deferred search shuffle", "DB$ ChangeZone | Origin$ Library | Destination$ Hand | Defined$ Remembered", true},
		{"NoLooking placement leg", "DB$ ChangeZone | Origin$ Library | Destination$ Hand | Defined$ Remembered | NoLooking$ True", false},
		{"explicit Shuffle$ True", "DB$ ChangeZone | Origin$ Library | Destination$ Hand | Defined$ Remembered | Shuffle$ True", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			source := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Sorcery\nOracle:x\n"), 0)
			revealed := h.g.AddObject(mkCard(t, "Name:Revealed\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
			under := h.g.AddObject(mkCard(t, "Name:Under\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
			h.g.SetZone(state.ZLibrary, 0, []state.ObjID{revealed.ID, under.ID})
			h.g.Obj(revealed.ID).Zone = state.ZLibrary
			h.g.Obj(under.ID).Zone = state.ZLibrary
			if h.g.Obj(revealed.ID).Zone != state.ZLibrary {
				t.Fatal("precondition: remembered card starts in the library")
			}
			ctx := &Ctx{Source: source.ID, Controller: 0, Remembered: []state.Target{{Obj: revealed.ID}}}
			ctx.Search.DeferredShuffle = tc.deferred
			Resolve(h, ctx, sa(t, tc.script))
			if got := h.g.Obj(revealed.ID).Zone; got != state.ZHand {
				t.Fatalf("remembered card zone = %s, want hand", got)
			}
			if n := definedRevealShuffles(h); n != 1 {
				t.Fatalf("%s emitted %d Shuffle events, want 1: %+v", tc.name, n, h.log)
			}
		})
	}
}
