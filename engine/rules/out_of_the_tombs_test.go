// Out of the Tombs' empty-library Draw replacement: an attempted draw while
// the library is empty returns a creature card from the graveyard to the
// battlefield (the mandatory hidden pick), and with no creature card to
// return the Remembered-gated LosesGame branch resolves the loss. The draw
// leaves no empty-library mark either way (CR 614.11).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// outOfTheTombsFixture deals seat 0 the real Out of the Tombs and, when
// withCreature is set, a Grizzly Bears. It puts the enchantment on the
// battlefield and (when present) moves the bear from hand or library to the
// graveyard, both with logged MoveZones. It returns the engine, the config and
// the two object ids (creature 0 when absent).
func outOfTheTombsFixture(t *testing.T, seed uint64, withCreature bool) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	tombs := mustCorpusCard(t, reg, "Out of the Tombs")
	if d := tombs.Link(); len(d) != 0 {
		t.Fatalf("link Out of the Tombs: %v", d)
	}
	deck := []*cards.Card{tombs}
	if withCreature {
		bears := mustCorpusCard(t, reg, "Grizzly Bears")
		if d := bears.Link(); len(d) != 0 {
			t.Fatalf("link Grizzly Bears: %v", d)
		}
		deck = append(deck, bears)
	}
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: reg.Tokens, NameUniverse: reg.Universe()})
	e := New(cfg)
	e.Advance()
	tombsID := moveNamedToBattlefield(t, e, 0, "Out of the Tombs")
	var bearID state.ObjID
	if withCreature {
		bearID = moveNamedToGraveyard(t, e, 0, "Grizzly Bears")
	}
	return e, cfg, tombsID, bearID
}

// moveNamedToGraveyard finds the named card in p's hand or library and moves it
// onto p's graveyard with a logged MoveZone.
func moveNamedToGraveyard(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, cand := range e.G.Zone(z, p) {
			if o := e.G.Obj(cand); o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: cand, From: z, To: state.ZGraveyard})
				return cand
			}
		}
	}
	t.Fatalf("%s was not dealt to seat %d", name, p)
	return 0
}

// TestOutOfTheTombsReturnsCreatureOnEmptyLibraryDraw pins both branches of
// the replacement and the ordinary draw its library gate admits.
func TestOutOfTheTombsReturnsCreatureOnEmptyLibraryDraw(t *testing.T) {
	t.Parallel()
	t.Run("creature returns", func(t *testing.T) {
		e, cfg, tombs, bear := outOfTheTombsFixture(t, 8350, true)
		o := e.G.Obj(tombs)
		if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
			t.Fatalf("precondition: Out of the Tombs is not on seat 0's battlefield: %+v", o)
		}
		preconditionOutOfTheTombsDrawLine(t, o)
		if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: Grizzly Bears is not in seat 0's graveyard: %+v", o)
		}
		cantLoseEmptyLibrary(t, e, 0)
		if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
			t.Fatalf("precondition: seat 0 library holds %d cards, want 0", n)
		}

		// The draw is replaced by the mandatory pick. Outside a resolution run
		// the engine answers the hidden pick with its deterministic stand-in
		// (the only eligible creature); the lockstep test drives the same pick
		// through the kernel's ask tape.
		effects.DrawFor(e, 0)
		if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("returned creature = %+v, want on the battlefield", e.G.Obj(bear))
		}
		if e.G.Players[0].DrewFromEmpty || countDrawEmpty(e, 0, 0) != 0 {
			t.Fatal("the replaced draw was marked from an empty library")
		}
		e.checkStateBased()
		if e.G.Players[0].Lost || e.G.Over {
			t.Fatalf("seat 0 lost=%v over=%v although the replacement returned a creature",
				e.G.Players[0].Lost, e.G.Over)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("nothing to return loses", func(t *testing.T) {
		e, cfg, tombs, _ := outOfTheTombsFixture(t, 8351, false)
		o := e.G.Obj(tombs)
		if o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
			t.Fatalf("precondition: Out of the Tombs is not on seat 0's battlefield: %+v", o)
		}
		preconditionOutOfTheTombsDrawLine(t, o)
		cantLoseEmptyLibrary(t, e, 0)
		if n := len(e.G.Zone(state.ZGraveyard, 0)); n == 0 {
			t.Fatal("precondition: seat 0's graveyard is empty, but the setup moved no cards there")
		}
		for _, id := range e.G.Zone(state.ZGraveyard, 0) {
			if f := e.G.Obj(id).Face(); f != nil && f.IsCreature() {
				t.Fatalf("precondition: a creature card %q is in the graveyard", f.Name)
			}
		}

		effects.DrawFor(e, 0)
		// The loss is the replacement's own LosesGame (immediate), not the
		// empty-library mark's next state-based check: no mark, yet Lost.
		if e.G.Players[0].DrewFromEmpty || countDrawEmpty(e, 0, 0) != 0 {
			t.Fatal("the body's loss was reported as an empty-library mark")
		}
		if !e.G.Players[0].Lost {
			t.Fatal("seat 0 did not lose through Out of the Tombs' LosesGame branch")
		}
		e.checkStateBased()
		replayCheck(t, e, cfg)
	})

	t.Run("non-empty library draws normally", func(t *testing.T) {
		e, cfg, tombs, bear := outOfTheTombsFixture(t, 8352, true)
		hand := len(e.G.Zone(state.ZHand, 0))
		effects.DrawFor(e, 0)
		if got := len(e.G.Zone(state.ZHand, 0)); got != hand+1 {
			t.Fatalf("ordinary draw: hand %d -> %d, want +1", hand, got)
		}
		if o := e.G.Obj(tombs); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("Out of the Tombs left the battlefield: %+v", e.G.Obj(tombs))
		}
		if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("the bear moved although the library was not empty: %+v", e.G.Obj(bear))
		}
		replayCheck(t, e, cfg)
	})
}

// preconditionOutOfTheTombsDrawLine asserts the parsed face really carries the
// empty-library Draw replacement the behaviour under test needs, so the test
// cannot pass vacuously on a fixture that lost its line.
func preconditionOutOfTheTombsDrawLine(t *testing.T, o *state.Object) {
	t.Helper()
	for i := range o.Face().Repls {
		r := &o.Face().Repls[i]
		if r.Event == "Draw" && r.With != nil {
			return
		}
	}
	t.Fatalf("%s carries no parsed bodied Event$ Draw line", o.Face().Name)
}
