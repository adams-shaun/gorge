package rules

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cr-g47: a Dig over ANOTHER player's library moves the card it takes with
// a Secret MoveZone. Secret's Player is the one seat redaction rule 1 hands
// the payload to (engine/view/redact.go), so it must name the seat that
// LOOKED -- the Dig's chooser (CR 701.29a; "look at the top card of target
// player's library") -- not the library's owner, who never saw the card
// (CR 401.2: no player may look at the cards in a library). The owner's Seat
// projection carries their own library as id+name CardViews
// (view.PlayerView.Library), so an ObjID alone names the card to them, and
// view.Describe reads the name off the live game: the owner's transcript line
// would say which card moved and where.

// digSecretMoveOf returns the first Secret MoveZone of id from the library
// logged at or after mark, fatal when there is none.
func digSecretMoveOf(t *testing.T, e *Engine, mark int, id state.ObjID, to state.Zone) events.Event {
	t.Helper()
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.From == state.ZLibrary && ev.To == to {
			if !ev.Secret {
				t.Fatalf("the dig's move of %d is not Secret: %+v", id, ev)
			}
			return ev
		}
	}
	t.Fatalf("no MoveZone of %d from library to %v after event %d", id, to, mark)
	return events.Event{}
}

// assertSecretMoveSeenOnlyBy checks one Secret library move as each seat is
// served it (view.RedactEventFor, Seat visibility, then view.Describe -- the
// host's eventBodiesFor pair): the looker gets the card, the other seat gets
// only the move's shape and a line that names no card.
func assertSecretMoveSeenOnlyBy(t *testing.T, e *Engine, ev events.Event, name string, looker, other state.PlayerID) {
	t.Helper()
	hidden := view.RedactEventFor(e.G, ev, other, view.Seat)
	hiddenLine := view.Describe(e.G, hidden)
	if hidden.Obj != 0 || strings.Contains(hiddenLine, name) || strings.Contains(hiddenLine, "#"+strconv.FormatUint(uint64(ev.Obj), 10)) {
		t.Fatalf("seat %d (did not look) is served the move as Obj %d, line %q: it names the card only seat %d looked at (CR 401.2)", other, hidden.Obj, hiddenLine, looker)
	}
	if hidden.Kind != events.MoveZone || hidden.From != ev.From || hidden.To != ev.To {
		t.Fatalf("seat %d's redacted move lost its shape: %+v (want MoveZone %v -> %v: a card visibly moved)", other, hidden, ev.From, ev.To)
	}
	seen := view.RedactEventFor(e.G, ev, looker, view.Seat)
	if seen.Obj != ev.Obj || !strings.Contains(view.Describe(e.G, seen), name) {
		t.Fatalf("the looker (seat %d) is served the move as Obj %d, line %q, want the card %d it looked at", looker, seen.Obj, view.Describe(e.G, seen), ev.Obj)
	}
}

// TestJacePlusTwoBottomedCardIsHiddenFromItsOwner: Jace, the Mind Sculptor's
// [+2] targeting seat 1, answered "put it on the bottom". Seat 1 sees that a
// card went from the top of its library to the bottom (the move's shape) but
// not which card: only Jace's controller looked at it.
func TestJacePlusTwoBottomedCardIsHiddenFromItsOwner(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	jaceCard := mustCorpusCard(t, reg, "Jace, the Mind Sculptor")
	plusTwo := jaceAbility(t, jaceCard, "AddCounter", 2)
	e, cfg, jace := walkerBoard(t, reg, "Jace, the Mind Sculptor")
	e.Advance()
	oppTop := e.G.Zone(state.ZLibrary, 1)[0]
	name := e.G.Obj(oppTop).Face().Name
	submitChoices(t, e, abilityOption(t, e, jace, plusTwo).Index)
	targetPlayer(t, e, 1)
	mark := len(e.L.Events)
	var d *decision.Decision
	for i := 0; i < 20; i++ {
		if d = e.Pending(); d == nil || d.ResumeKind == "dig" || d.Kind != decision.KPriority {
			break
		}
		passOnce(t, e)
	}
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "dig" || len(d.Options) != 1 || d.Options[0].Obj != oppTop {
		t.Fatalf("no Jace [+2] dig ask over seat 1's top card %d: %+v", oppTop, d)
	}
	submitChoices(t, e, 0) // put it on the bottom
	passUntilStackEmpty(t, e, 20)
	ev := digSecretMoveOf(t, e, mark, oppTop, state.ZLibrary)
	assertSecretMoveSeenOnlyBy(t, e, ev, name, 0, 1)
	replayCheck(t, e, cfg)
}

// TestGontiFaceDownExileIsHiddenFromTheLibraryOwner: Gonti, Lord of
// Luxury's enter trigger on seat 1 exiles one of seat 1's top four cards
// face down. Seat 1's projection already withholds the face-down card's
// face (WithMayLook$: only Gonti's controller may look); the Secret move
// that put it there must not name it to seat 1 either.
func TestGontiFaceDownExileIsHiddenFromTheLibraryOwner(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	gonti := lookup(t, reg, "Gonti, Lord of Luxury")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{gonti}, nil)
	moveByName(t, e, 0, "Gonti, Lord of Luxury", state.ZHand)
	addMana(t, e, 0, "BBBB")
	castNamed(t, e, "Gonti, Lord of Luxury")
	var d *decision.Decision
	mark := 0
	for i := 0; i < 40; i++ {
		d = e.Pending()
		if d == nil || d.ResumeKind == "dig" {
			break
		}
		switch d.Kind {
		case decision.KTarget:
			targetPlayer(t, e, 1)
			mark = len(e.L.Events)
		case decision.KPriority:
			passOnce(t, e)
		default:
			t.Fatalf("unexpected decision before Gonti's dig ask: %+v", d)
		}
	}
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "dig" || len(d.Options) != 4 {
		t.Fatalf("no Gonti dig ask over seat 1's top four: %+v", d)
	}
	taken := d.Options[0].Obj
	name := e.G.Obj(taken).Face().Name
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(taken); o == nil || o.Zone != state.ZExile || !o.FaceDown || o.Owner != 1 {
		t.Fatalf("precondition: Gonti's pick is not face down in seat 1's exile: %+v", o)
	}
	ev := digSecretMoveOf(t, e, mark, taken, state.ZExile)
	assertSecretMoveSeenOnlyBy(t, e, ev, name, 0, 1)
	replayCheck(t, e, cfg)
}

// TestHideawayExileIsSeatOnesSecretNotSeatZeros: Clive's Hideaway entering
// under SEAT 1 -- its controller looks at its own top four and exiles one
// face down (CR 702.75a). The Secret exile must be seat 1's: today it carries
// no Player at all, so it reads as seat 0's secret and seat 0 is served the
// hidden card.
func TestHideawayExileIsSeatOnesSecretNotSeatZeros(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	clive := lookup(t, reg, "Clive's Hideaway")
	deck := make([]*cards.Card, 40)
	for i := range deck {
		deck[i] = clive
	}
	cfg := seatZeroStart(Config{Seed: 818, Names: []string{"other", "clive"}, Decks: [][]*cards.Card{deck, deck}})
	e := New(cfg)
	id := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	e.putTriggersOnStack()
	mark := len(e.L.Events)
	kr6ResolveTop(e)
	pick := e.Pending()
	if pick == nil || pick.Kind != decision.KChoose || pick.Player != 1 || len(pick.Options) != 4 {
		t.Fatalf("Hideaway pick = %+v, want seat 1's four-card choice", pick)
	}
	chosen := pick.Options[1].Obj
	submitChoices(t, e, pick.Options[1].Index)
	if bottom := e.Pending(); bottom != nil && bottom.Kind == decision.KArrange {
		submitChoices(t, e, 2, 0, 1)
	}
	if o := e.G.Obj(chosen); o == nil || o.Zone != state.ZExile || !o.FaceDown {
		t.Fatalf("precondition: the Hideaway pick is not face down in exile: %+v", o)
	}
	ev := digSecretMoveOf(t, e, mark, chosen, state.ZExile)
	assertSecretMoveSeenOnlyBy(t, e, ev, "Clive's Hideaway", 1, 0)
	replayCheck(t, e, cfg)
}
