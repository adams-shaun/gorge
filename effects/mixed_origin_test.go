package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// corpusObjectInSeat is corpusObject for a named seat: the card joins the
// game under that controller and is then placed in the named zone, so a test
// can stage hidden-zone candidates for a player other than the resolver.
func corpusObjectInSeat(t *testing.T, reg *cards.Registry, g *state.Game,
	name string, seat state.PlayerID, z state.Zone) *state.Object {
	t.Helper()
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	o := g.AddObject(c, seat)
	g.SetZone(z, seat, []state.ObjID{o.ID})
	// Re-read through g.Obj: AddObject appends to g.Objs and may reallocate,
	// so the pointer AddObject returned can go stale the moment the next
	// object joins. The Zone field itself must be written on the live copy --
	// the engine's movers read it, not the zone lists alone.
	live := g.Obj(o.ID)
	live.Zone = z
	return live
}

// zoneOfObj reads an object's live zone back through the game's own table --
// never through a pointer held across another AddObject.
func zoneOfObj(g *state.Game, id state.ObjID) state.Zone {
	o := g.Obj(id)
	if o == nil {
		return 0
	}
	return o.Zone
}

// TestDreamsOfSteelAndOilMixedOriginExilesBothChosenCards closes the row the
// mixed-origin Note once covered: Dreams of Steel and Oil's DBExileBoth is a
// real corpus `ChangeZone | Defined$ RememberedCard | Origin$ Graveyard,Hand`
// whose two ChooseCard answers must BOTH exile. Before the fix the unmodelled
// RememberedCard selector fell through Defined to the source default, the
// Origin$ precondition skipped it, and the resolution emitted one replay-note
// and moved nothing. Precondition: the two staged candidates sit in the two
// origin zones, are distinct, and the resolution's Remembered names both.
func TestDreamsOfSteelAndOilMixedOriginExilesBothChosenCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Dreams of Steel and Oil")
	if !ok {
		t.Fatal("corpus has no Dreams of Steel and Oil")
	}
	db := cards.ResolveSVar(card.Faces[0].SVars, "DBExileBoth")
	if db == nil || db.Params["Origin"] != "Graveyard,Hand" ||
		db.Params["Defined"] != "RememberedCard" || db.Params["Destination"] != "Exile" {
		t.Fatalf("Dreams' compiled mixed-origin exile drifted: %+v", db)
	}

	g := state.NewGame(names(2))
	source := corpusObject(t, reg, g, "Dreams of Steel and Oil")
	handCard := corpusObjectInSeat(t, reg, g, "Storm Crow", 1, state.ZHand)
	graveCard := corpusObjectInSeat(t, reg, g, "Storm Crow", 1, state.ZGraveyard)
	if handCard.ID == graveCard.ID {
		t.Fatal("the staged candidates must be distinct objects")
	}
	if zoneOfObj(g, handCard.ID) != state.ZHand || zoneOfObj(g, graveCard.ID) != state.ZGraveyard {
		t.Fatalf("precondition: hand=%s grave=%s, want the two origin zones",
			zoneOfObj(g, handCard.ID), zoneOfObj(g, graveCard.ID))
	}

	h := &askHost{}
	h.g = g
	Resolve(h, &Ctx{Source: source.ID, Controller: 0,
		Remembered: []state.Target{{Obj: handCard.ID}, {Obj: graveCard.ID}}}, db)
	if zoneOfObj(g, handCard.ID) != state.ZExile || zoneOfObj(g, graveCard.ID) != state.ZExile {
		t.Fatalf("mixed-origin exile moved nothing loudly: hand=%s grave=%s, want both in Exile",
			zoneOfObj(g, handCard.ID), zoneOfObj(g, graveCard.ID))
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "mixed ChangeZone Origin") {
			t.Fatalf("spurious mixed-origin note beside a resolution that moved both cards: %+v", ev)
		}
	}
}

// TestPhantasmalExtractionChosenCardExilesWithoutMixedOriginNote pins the
// ChosenCard half of the same row: Phantasmal Extraction's DBChangeZone
// already names its object (the ChooseCard answer), Origin$ Hand,Graveyard is
// only a precondition over it, so the resolution must move the chosen card
// WITHOUT the old unconditional "cannot choose cards from mixed ChangeZone
// Origin$" note -- the chooser the note claimed missing is the ChooseCard ask
// the chain already answered.
func TestPhantasmalExtractionChosenCardExilesWithoutMixedOriginNote(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Phantasmal Extraction")
	if !ok {
		t.Fatal("corpus has no Phantasmal Extraction")
	}
	db := cards.ResolveSVar(card.Faces[0].SVars, "DBChangeZone")
	if db == nil || db.Params["Origin"] != "Hand,Graveyard" ||
		db.Params["Defined"] != "ChosenCard" || db.Params["Destination"] != "Exile" {
		t.Fatalf("Phantasmal Extraction's compiled mixed-origin exile drifted: %+v", db)
	}

	g := state.NewGame(names(2))
	source := corpusObject(t, reg, g, "Phantasmal Extraction")
	chosen := corpusObjectInSeat(t, reg, g, "Storm Crow", 1, state.ZHand)
	if zoneOfObj(g, chosen.ID) != state.ZHand {
		t.Fatalf("precondition: chosen card in %s, want the opponent's hand", zoneOfObj(g, chosen.ID))
	}

	h := &askHost{}
	h.g = g
	Resolve(h, &Ctx{Source: source.ID, Controller: 0,
		Chosen:      []state.Target{{Obj: chosen.ID}},
		ChosenValid: true}, db)
	if zoneOfObj(g, chosen.ID) != state.ZExile {
		t.Fatalf("chosen card = %s, want Exile", zoneOfObj(g, chosen.ID))
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "mixed ChangeZone Origin") {
			t.Fatalf("spurious mixed-origin note beside a resolution that moved its chosen card: %+v", ev)
		}
	}
}

// TestMixedOriginNoteStillFiresWhenNothingMoves keeps the row's loud fallback
// honest after the narrowing: a mixed-Hand origin whose resolution moves
// nothing (here a Defined$ Self sitting outside both named origins) still
// emits the diagnostic -- and no MoveZone -- so a dead resolution is never
// silent.
func TestMixedOriginNoteStillFiresWhenNothingMoves(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame(names(2))
	source := corpusObject(t, reg, g, "Storm Crow")
	if zoneOfObj(g, source.ID) != state.ZBattlefield {
		t.Fatalf("precondition: source in %s, want the battlefield", zoneOfObj(g, source.ID))
	}
	db := sa(t, "DB$ ChangeZone | Defined$ Self | Origin$ Hand,Graveyard | Destination$ Exile")
	if db.Params["Origin"] != "Hand,Graveyard" {
		t.Fatalf("precondition: parsed origin %q", db.Params["Origin"])
	}

	h := &askHost{}
	h.g = g
	Resolve(h, &Ctx{Source: source.ID, Controller: 0}, db)
	if zoneOfObj(g, source.ID) != state.ZBattlefield {
		t.Fatalf("an out-of-origin Defined$ Self must not move: now in %s", zoneOfObj(g, source.ID))
	}
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "mixed ChangeZone Origin$ Hand,Graveyard") {
			found = true
		}
		if ev.Kind == events.MoveZone {
			t.Fatalf("nothing was eligible to move: unexpected %+v", ev)
		}
	}
	if !found {
		t.Fatalf("a nothing-moves mixed-origin resolution emitted no diagnostic: %+v", h.log)
	}
}

// TestMixedOriginChooserAskIsAddressedToTheHandOwner pins the per-zone
// hidden-ness half of the row: Kastral's optional Bird picker spans the
// controller's OWN hand and graveyard, so the one private option list must be
// addressed to the hand cards' owner -- the ask is private to that seat, and
// no other seat's channel can carry it.
func TestMixedOriginChooserAskIsAddressedToTheHandOwner(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	kastral, ok := reg.Lookup("Kastral, the Windcrested")
	if !ok {
		t.Fatal("corpus has no Kastral, the Windcrested")
	}
	db := cards.ResolveSVar(kastral.Faces[0].SVars, "DBChangeZone")
	if db == nil || db.Params["Origin"] != "Hand,Graveyard" {
		t.Fatalf("Kastral's compiled mixed-origin picker drifted: %+v", db)
	}

	g := state.NewGame(names(2))
	source := corpusObject(t, reg, g, "Kastral, the Windcrested")
	handBird := corpusObjectInSeat(t, reg, g, "Storm Crow", 0, state.ZHand)
	graveBird := corpusObjectInSeat(t, reg, g, "Storm Crow", 0, state.ZGraveyard)
	if zoneOfObj(g, handBird.ID) != state.ZHand || zoneOfObj(g, graveBird.ID) != state.ZGraveyard {
		t.Fatalf("precondition: hand=%s grave=%s",
			zoneOfObj(g, handBird.ID), zoneOfObj(g, graveBird.ID))
	}

	h := &askHost{}
	h.g = g
	Resolve(h, &Ctx{Source: source.ID, Controller: 0}, db)
	if h.asked == nil {
		t.Fatal("the mixed-origin chooser posed no ask")
	}
	if h.asked.Player != 0 {
		t.Fatalf("mixed-origin ask addressed to seat %d, want the hidden hand's owner (0)",
			h.asked.Player)
	}
}
