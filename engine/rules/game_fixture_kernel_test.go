package rules

// game_fixture_kernel_test.go: a shared fixture. Every board here is
// a real engine reached through logged events, so a test can answer each
// posed decision with Submit and replay-check the result.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// fixtureGame builds a len(decks)-seat engine at seat 0's turn-1 Main1. Seat p's
// deck is decks[p] followed by Mountains up to 40 cards; every fixture card
// is a real deck card, so moving it with moveNamedTo is replay-traceable.
func fixtureGame(t *testing.T, seed uint64, decks ...[]*cards.Card) (*Engine, Config) {
	t.Helper()
	names := make([]string, len(decks))
	for i := range names {
		names[i] = fmt.Sprintf("p%d", i)
	}
	build := func(s uint64) Config {
		ds := make([][]*cards.Card, len(decks))
		for p, d := range decks {
			ds[p] = append(append([]*cards.Card(nil), d...), mountainDeck(t, 40-len(d))...)
		}
		return Config{Seed: s, Names: names, Decks: ds, Tokens: map[string]*cards.Card{}}
	}
	cfg := seatZeroStart(build(seed))
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// cardsOfSrcs parses inline card scripts (never corpus text).
func cardsOfSrcs(t *testing.T, srcs ...string) []*cards.Card {
	t.Helper()
	out := make([]*cards.Card, len(srcs))
	for i, s := range srcs {
		out[i] = card(t, s)
	}
	return out
}

// creatureLineSrc is a vanilla creature script named name.
func creatureLineSrc(name string) string {
	return "Name:" + name + "\nManaCost:2\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
}

// findNamedInZone returns the first object named name owned by p in zone z, or 0.
func findNamedInZone(e *Engine, p state.PlayerID, z state.Zone, name string) state.ObjID {
	for _, id := range e.G.Zone(z, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	return 0
}

// moveNamedTo moves seat p's first card named name from its library, hand or
// exile (in that order) to zone to with a logged MoveZone. A card already
// in zone to is returned unmoved.
func moveNamedTo(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone) state.ObjID {
	t.Helper()
	if id := findNamedInZone(e, p, to, name); id != 0 {
		return id
	}
	for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZExile} {
		if z == to {
			continue
		}
		if id := findNamedInZone(e, p, z, name); id != 0 {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
			e.pending = nil
			return id
		}
	}
	t.Fatalf("seat %d has no %q in library, hand or exile", p, name)
	return 0
}

// emptyHand exiles every card in seat p's hand (logged), so a test can
// then put an exact hand together with moveNamedTo.
func emptyHand(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, p)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZExile})
	}
	e.pending = nil
}

// libraryTopOrder reorders seat p's library so ids sit on top, in order (a
// logged LibraryOrder).
func libraryTopOrder(t *testing.T, e *Engine, p state.PlayerID, ids ...state.ObjID) {
	t.Helper()
	top := make(map[state.ObjID]bool, len(ids))
	for _, id := range ids {
		top[id] = true
	}
	lib := append([]state.ObjID(nil), ids...)
	for _, id := range e.G.Zone(state.ZLibrary, p) {
		if !top[id] {
			lib = append(lib, id)
		}
	}
	if len(lib) != len(e.G.Zone(state.ZLibrary, p)) {
		t.Fatalf("libraryTopOrder: %v are not all in seat %d's library", ids, p)
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: p, IDs: lib})
	e.pending = nil
}

// castNamedSpell funds seat 0 with mana, casts the named card from seat 0's hand
// (answering a player-target ask with seat target when target >= 0, else
// option 0; a modal spell takes its first mode; a hybrid or Phyrexian
// symbol is paid with mana), passes priority until a non-priority decision is posed, and
// returns it -- nil once the stack has drained with nothing posed.
func castNamedSpell(t *testing.T, e *Engine, name, mana string, target int) *decision.Decision {
	t.Helper()
	id := findNamedInZone(e, 0, state.ZHand, name)
	if id == 0 {
		t.Fatalf("%q is not in seat 0's hand", name)
	}
	e.pending = nil
	addMana(t, e, 0, mana)
	submitChoices(t, e, castOptionFor(t, e, id).Index)
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		switch {
		case d.Kind == decision.KTarget:
			idx := 0
			if target >= 0 {
				idx = -1
				for _, o := range d.Options {
					if o.Kind == "player" && o.Player == state.PlayerID(target) {
						idx = o.Index
					}
				}
				if idx < 0 {
					t.Fatalf("seat %d not offered as a target: %+v", target, d.Options)
				}
			}
			submitChoices(t, e, idx)
		case d.Kind == decision.KModes:
			// A modal spell's announced mode: the first one.
			submitChoices(t, e, d.Options[0].Index)
		case d.Kind == decision.KChoose && len(d.Options) > 0 && strings.HasPrefix(d.Options[0].Kind, "pay_"):
			// A hybrid/Phyrexian symbol's payment choice: pay with mana.
			submitChoices(t, e, d.Options[0].Index)
		default:
			t.Fatalf("unexpected cast-time decision %+v", d)
		}
	}
	return nextPending(t, e)
}

// nextPending passes priority until a non-priority decision is posed and returns
// it, or returns nil once the stack is empty at a priority decision.
func nextPending(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 60 && !e.G.Over; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending")
		}
		if d.Kind != decision.KPriority {
			return d
		}
		if len(e.G.Stack) == 0 {
			return nil
		}
		submitChoices(t, e, tapePassIndex(d))
	}
	t.Fatal("the stack never settled")
	return nil
}

// answerChoices submits choices against the pending decision and returns the
// next non-priority decision (nil once the stack drains).
func answerChoices(t *testing.T, e *Engine, choices ...int) *decision.Decision {
	t.Helper()
	submitChoices(t, e, choices...)
	return nextPending(t, e)
}

// optionKindOrFail returns the index of d's first option of kind k.
func optionKindOrFail(t *testing.T, d *decision.Decision, k string) int {
	t.Helper()
	for _, o := range d.Options {
		if o.Kind == k {
			return o.Index
		}
	}
	t.Fatalf("no %q option in %+v", k, d.Options)
	return -1
}

// optionObjIDs is the object of every option of d, in offered order.
func optionObjIDs(d *decision.Decision) []state.ObjID {
	out := make([]state.ObjID, 0, len(d.Options))
	for _, o := range d.Options {
		out = append(out, o.Obj)
	}
	return out
}

// countKindFrom counts events of kind logged at or after index from.
func countKindFrom(e *Engine, from int, kind events.Kind) int {
	n := 0
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == kind {
			n++
		}
	}
	return n
}

// zoneOfObjID is obj's current zone.
func zoneOfObjID(e *Engine, obj state.ObjID) state.Zone { return e.G.Obj(obj).Zone }
