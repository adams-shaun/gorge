package rules

// cr-g61: CR 401.2 -- "Players can't look at or change the order of cards in
// a library" -- covers the library's owner too. Every Secret LibraryOrder
// carries the library's COMPLETE new order in IDs (events.Apply needs it),
// and view.RedactEvent used to hand that payload to the event's Player, the
// owner. With the owner's projection listing the library's cards by id
// (PlayerView.Library), the ids named every card of the library in order.
// The owner is served the event's shape, exactly as every other seat, the
// public spectator and the omniscient spectator already are.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// libraryOrdersAfter is every LibraryOrder for seat p's library after mark.
func libraryOrdersAfter(e *Engine, mark int, p state.PlayerID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.LibraryOrder && ev.Player == p {
			out = append(out, ev)
		}
	}
	return out
}

// assertLibraryOrderShapeForAll checks that every seat, the public
// spectator and the omniscient spectator are served ev as its shape: no
// IDs, no Obj, the same Kind/Player/Secret, and the same transcript line the
// owner was served before (the line never named a card).
func assertLibraryOrderShapeForAll(t *testing.T, e *Engine, ev events.Event, wantLine string) {
	t.Helper()
	if len(ev.IDs) < 30 || !ev.Secret {
		t.Fatalf("precondition: the raw LibraryOrder %+v is not a Secret whole-library order", ev)
	}
	type served struct {
		who string
		got events.Event
	}
	var all []served
	for s := range e.G.Players {
		all = append(all, served{"seat " + string(rune('0'+s)), view.RedactEventFor(e.G, ev, state.PlayerID(s), view.Seat)})
	}
	all = append(all,
		served{"public", view.RedactEventFor(e.G, ev, view.NoSeat, view.Public)},
		served{"omniscient", view.RedactEventFor(e.G, ev, view.NoSeat, view.Omniscient)})
	for _, s := range all {
		if len(s.got.IDs) != 0 || s.got.Obj != 0 || s.got.Kind != events.LibraryOrder || s.got.Player != ev.Player || !s.got.Secret {
			t.Fatalf("%s is served %+v for seat %d's LibraryOrder; want its shape only (CR 401.2)", s.who, s.got, ev.Player)
		}
		if line := view.Describe(e.G, s.got); line != wantLine {
			t.Fatalf("%s's transcript line = %q, want %q", s.who, line, wantLine)
		}
	}
}

// TestPonderLibraryOrderIsHiddenFromItsOwner: seat 0 casts Ponder (repo
// decks uw-control, ur-delver, ...), puts the top three back reversed and
// declines the shuffle. The owner knew the three cards it arranged -- the
// KArrange ask named them, and the owner answered it -- but not the rest of
// the library, which the LibraryOrder's IDs list in full.
func TestPonderLibraryOrderIsHiddenFromItsOwner(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	ponder := lookup(t, reg, "Ponder")
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{ponder}, nil)
	moveByName(t, e, 0, "Ponder", state.ZHand)
	addMana(t, e, 0, "U")
	top3 := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)[:3]...)
	castNamed(t, e, "Ponder")
	mark := len(e.L.Events)
	var d *decision.Decision
	for i := 0; i < 20; i++ {
		if d = e.Pending(); d == nil || d.Kind != decision.KPriority {
			break
		}
		passOnce(t, e)
	}
	if d == nil || d.Kind != decision.KArrange || d.Player != 0 || len(d.Options) != 3 {
		t.Fatalf("no Ponder arrange ask for seat 0: %+v", d)
	}
	// The owner's legitimate knowledge is the ask itself: it names exactly
	// the top three cards, and that is untouched by the event redaction.
	for i, o := range d.Options {
		if o.Obj != top3[i] {
			t.Fatalf("Ponder's arrange option %d names %d, want the top card %d", i, o.Obj, top3[i])
		}
	}
	submitChoices(t, e, 2, 1, 0)
	if sd := e.Pending(); sd != nil && sd.Kind == decision.KChoose && sd.ResumeKind == "arrange_mayshuffle" {
		no := -1
		for _, o := range sd.Options {
			if o.Kind == "no" {
				no = o.Index
			}
		}
		if no < 0 {
			t.Fatalf("no 'no' option on the may-shuffle ask: %+v", sd.Options)
		}
		submitChoices(t, e, no)
	}
	passUntilStackEmpty(t, e, 20)
	orders := libraryOrdersAfter(e, mark, 0)
	if len(orders) != 1 {
		t.Fatalf("want exactly one LibraryOrder for seat 0's Ponder, got %+v", orders)
	}
	ev := orders[0]
	if ev.IDs[0] != top3[2] || ev.IDs[1] != top3[1] || ev.IDs[2] != top3[0] {
		t.Fatalf("precondition: the LibraryOrder does not put the three back reversed: %v", ev.IDs[:3])
	}
	assertLibraryOrderShapeForAll(t, e, ev, "a rearranges the top of their library")
	replayCheck(t, e, cfg)
}

// TestGontiRemainderOrderIsHiddenFromTheLibraryOwner: Gonti, Lord of
// Luxury's enter trigger looks at seat 1's top four, exiles one face down
// and puts the rest on the bottom in a random order (one LibraryOrder for
// seat 1's library). Seat 1 never saw those cards, but the order's IDs
// told it which three Gonti's controller looked at, and where every other
// card of its library sits.
func TestGontiRemainderOrderIsHiddenFromTheLibraryOwner(t *testing.T) {
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
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	orders := libraryOrdersAfter(e, mark, 1)
	if len(orders) != 1 {
		t.Fatalf("want exactly one LibraryOrder for seat 1's library after Gonti, got %+v", orders)
	}
	assertLibraryOrderShapeForAll(t, e, orders[0], "b rearranges the top of their library")
	replayCheck(t, e, cfg)
}
