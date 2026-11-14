// The makeslice regression the MBC Commander draft seeds exposed: a
// countered commander spell whose counter sends it to its owner's LIBRARY
// meets the CR 903.9b command-zone replacement (the owner accepts, the
// commander's spell goes to the command zone instead of the library) while
// the owner's library is EMPTY. The library placement helpers then size
// their `rest` buffer from len(moved) minus a library that never received
// the card, and the negative cap panicked the whole game
// (makeslice: cap out of range). One test per placement helper:
//
//   - libraryOrderPlacement, the effCounter Destination$ TopOfLibrary shape
//     (Memory Lapse, engine/effects/resolve.go).
//   - libraryOrderPlacementAt, the effChangeZone LibraryPosition$ shape
//     (the object-target ChangeZone tail, engine/effects/zone_change.go).
//
// Both reuse the commander_zone_replacement_test.go posture: a Commander
// game, seat 0's cheap commander cast from the command zone and countered by
// seat 1, the owner's library emptied beforehand, and the owner answering
// the KCommanderZone ask with an accept.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cmdCrumpleKinSrc is the ChangeZone placement shape (the object-target
// ChangeZone tail that ends in libraryOrderPlacementAt): a plain counter
// whose SubAbility$ DBChange then moves the countered spell (Defined$
// Targeted) to its owner's library at LibraryPosition$ 0. The Counter leg
// buries the commander in its graveyard for real (CR 903.9a: a graveyard
// destination is never replaced at move time), so the DBChange's
// graveyard->library move is the one that meets the CR 903.9b replacement --
// parked, the commander never reaching the library -- and the ChangeZone
// placement then runs over a moved card that is on its way to the command
// zone, not in the library. The counter card itself resolves to the
// graveyard, so the effCounter library placement never runs and this test
// exercises the ChangeZone helper alone.
const cmdCrumpleKinSrc = `Name:Crumple Kin
ManaCost:U U
Types:Instant
A:SP$ Counter | ValidTgts$ Spell | SubAbility$ DBBury | SpellDescription$ Counter target spell. Then put it on top of its owner's library.
SVar:DBBury:DB$ ChangeZone | Origin$ Graveyard | Destination$ Library | LibraryPosition$ 0 | Defined$ Targeted
Oracle:Counter target spell. Then put it on top of its owner's library.
`

// cmdCounterGame builds the two-seat Commander game both tests share: seat 0
// leads with the decision-free Beatstick commander, seat 1's deck carries the
// given extra cards (the counterspells) ahead of its mountains, and the game
// is driven to turn 1 seat 0 Main1 -- the point a command-zone cast is
// offered. It returns the counterspell's seat so the caller can name the
// setup and the protagonist commander's ObjID.
func cmdCounterGame(t *testing.T, seed uint64, seat1 []*cards.Card) (*Engine, Config, state.ObjID) {
	t.Helper()
	deck0 := append([]*cards.Card{card(t, commanderBeatstickSrc)}, mountainDeck(t, 39)...)
	deck1 := append(append([]*cards.Card(nil), seat1...), mountainDeck(t, 40-len(seat1))...)
	cfg := Config{Seed: seed, Names: []string{"a", "b"},
		Decks:      [][]*cards.Card{deck0, deck1},
		Commanders: [][]int{{0}, nil}, StartingLife: 40, Format: FormatCommander}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()
	driveToStep(t, e, 1, 0, state.StepMain1)
	return e, cfg, e.G.Players[0].Commanders[0]
}

// cmdZoneAskPasses answers priority decisions until the CR 903.9
// KCommanderZone ask is posed -- the park-and-continue ask the commander
// replacement raises while the counter resolves -- and fails on anything
// else, so a mis-shaped resolution cannot slip past the test's own
// assertions.
func cmdZoneAskPasses(t *testing.T, e *Engine, limit int) {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while waiting for the commander-zone ask (stack depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KCommanderZone {
			return
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision waiting for the commander-zone ask: %+v", d)
		}
		miscPass(t, e)
	}
	t.Fatal("the commander-zone ask never arrived")
}

// cmdCounterTarget casts seat 1's counter (already funded and in hand) at the
// commander spell sitting on the stack, and returns once the target answer
// is in -- the point both tests pin their preconditions at.
func cmdCounterTarget(t *testing.T, e *Engine, counter, cmd state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected seat 1's priority after the pass, got %+v", d)
	}
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == counter {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("no cast option for the counter %d: %+v", counter, d.Options)
	}
	submitChoices(t, e, castIdx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision for the counter, got %+v", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Obj == cmd {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("the commander spell %d was not offered as the counter's target: %+v", cmd, d.Options)
	}
	submitChoices(t, e, tIdx)
}

// assertCounteredCommanderInCommandZone is the acceptance tail both tests
// share: the owner accepted, so the commander's spell went from the stack
// straight to the command zone -- never into a library or graveyard, both of
// which would mean the CR 903.9b replacement did not run. The ask and the
// counteredFrom->command MoveZone travel in the log, and exactly one ask was
// posed. counteredFrom is where the countered spell lay when the replacement
// met it: the stack for the effCounter library destination (the parked move),
// the graveyard for the ChangeZone shape (CR 903.9a: the counter buries it
// there for real first).
func assertCounteredCommanderInCommandZone(t *testing.T, e *Engine, cmd state.ObjID, counteredFrom state.Zone) {
	t.Helper()
	if o := e.G.Obj(cmd); o == nil || o.Zone != state.ZCommand {
		t.Fatalf("accepted commander = %+v, want it in the command zone", o)
	}
	if !contains(e.G.Zone(state.ZCommand, 0), cmd) {
		t.Fatalf("command zone = %v, want the commander %d", e.G.Zone(state.ZCommand, 0), cmd)
	}
	if contains(e.G.Zone(state.ZLibrary, 0), cmd) {
		t.Fatal("the commander ended up in seat 0's library: the CR 903.9b replacement did not run")
	}
	if contains(e.G.Zone(state.ZGraveyard, 0), cmd) {
		t.Fatal("the commander ended up in seat 0's graveyard: the CR 903.9b replacement did not run")
	}
	askSeen, moveSeen := false, false
	for _, ev := range e.L.Events {
		if ev.Kind == events.DecisionAsk && ev.Text == string(decision.KCommanderZone) {
			askSeen = true
		}
		if ev.Kind == events.MoveZone && ev.Obj == cmd && ev.From == counteredFrom && ev.To == state.ZCommand {
			moveSeen = true
		}
	}
	if !askSeen || !moveSeen {
		t.Fatalf("log missing ask/move: KCommanderZone ask=%v, stack->command MoveZone=%v", askSeen, moveSeen)
	}
}

// TestCounteredCommanderToCommandZone is the libraryOrderPlacement shape
// (Memory Lapse's Destination$ TopOfLibrary): the countered commander spell
// would go on top of its owner's library, the owner accepts the CR 903.9b
// replacement instead, and the placement helper must tolerate a moved card
// that never reached the library -- with the library EMPTY, the case that
// panicked the draft seeds. Everything passes with the library unchanged and
// the commander in the command zone.
func TestCounteredCommanderToCommandZone(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	lapse, ok := reg.Lookup("Memory Lapse")
	if !ok {
		t.Fatal("missing corpus card \"Memory Lapse\"")
	}
	e, _, cmd0 := cmdCounterGame(t, 5, []*cards.Card{lapse})
	counter := miscMoveByName(t, e, 1, "Memory Lapse", state.ZHand)
	cantLoseEmptyLibrary(t, e, 0)
	addMana(t, e, 1, "UUU")

	// PRECONDITION: seat 0's library is empty BEFORE the counter resolves,
	// so the placement helper sizes rest from a library that can hold
	// nothing -- the cap arithmetic the regression is about. A vacuous
	// setup (cards left in the library) would never panic and must fail
	// loudly here.
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("precondition: seat 0's library holds %d cards, want 0", n)
	}

	// Cast the commander from the command zone; it stays ON THE STACK as
	// the counter's target (precondition: a countered SPELL, not a
	// battlefield permanent).
	opt := commanderCastOption(e, cmd0)
	if opt == nil {
		t.Fatalf("no command-zone cast option for the commander: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	miscPass(t, e) // hand priority to seat 1
	cmdCounterTarget(t, e, counter, cmd0)
	if o := e.G.Obj(cmd0); o == nil || o.Zone != state.ZStack {
		t.Fatalf("precondition: commander = %+v, want its spell on the stack as the counter's target", o)
	}

	cmdZoneAskPasses(t, e, 30)
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's commander_zone decision", d)
	}
	submit(t, e, 0) // accept: into the command zone

	assertCounteredCommanderInCommandZone(t, e, cmd0, state.ZStack)
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("seat 0's library after the accept = %d cards, want 0 (unchanged)", n)
	}
	assertAskCount(t, e.L, 1)
}

// TestChangeZoneCommanderToCommandZone is the libraryOrderPlacementAt shape
// (the object-target ChangeZone tail): Crumple Kin counters the commander
// spell to its owner's HAND -- so the CR 903.9b replacement parks the
// commander and asks the owner -- and its SubAbility$ DBChange then moves
// the countered spell ITSELF (Defined$ Targeted) to seat 0's library at
// LibraryPosition$ 0, which is empty. That move is parked by the same
// replacement (deduped against the outstanding ask), and the ChangeZone
// placement helper must tolerate a moved card that is on its way to the
// command zone, not in the library -- the empty-library case that panicked
// the draft seeds. The counter card itself resolves to the graveyard, so
// the effCounter library placement never runs and this test exercises the
// ChangeZone helper alone.
func TestChangeZoneCommanderToCommandZone(t *testing.T) {
	t.Parallel()
	e, _, cmd0 := cmdCounterGame(t, 6, []*cards.Card{card(t, cmdCrumpleKinSrc)})
	counter := miscMoveByName(t, e, 1, "Crumple Kin", state.ZHand)
	cantLoseEmptyLibrary(t, e, 0)
	addMana(t, e, 1, "UUU")

	// PRECONDITION: seat 0's library is empty BEFORE the counter resolves,
	// so the placement helper sizes rest from a library that can hold
	// nothing -- the cap arithmetic the regression is about. A vacuous
	// setup (cards left in the library) would never panic and must fail
	// loudly here.
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("precondition: seat 0's library holds %d cards, want 0", n)
	}

	opt := commanderCastOption(e, cmd0)
	if opt == nil {
		t.Fatalf("no command-zone cast option for the commander: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	miscPass(t, e)
	cmdCounterTarget(t, e, counter, cmd0)
	if o := e.G.Obj(cmd0); o == nil || o.Zone != state.ZStack {
		t.Fatalf("precondition: commander = %+v, want its spell on the stack as the counter's target", o)
	}

	cmdZoneAskPasses(t, e, 30)
	if d := e.Pending(); d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's commander_zone decision", d)
	}
	submit(t, e, 0) // accept: into the command zone

	assertCounteredCommanderInCommandZone(t, e, cmd0, state.ZGraveyard)
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("seat 0's library after the accept = %d cards, want 0 (unchanged)", n)
	}
	// The counter card itself resolved and went to seat 1's graveyard --
	// only the COUNTERED spell was aimed at a library.
	if o := e.G.Obj(counter); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("resolved Crumple Kin = %+v, want graveyard", o)
	}
	assertAskCount(t, e.L, 1)
}
