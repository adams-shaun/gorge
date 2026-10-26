package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

var millSnapshotFillers = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta", "Theta"}

// millSnapshotBoard builds seat 0's library, top first, as the carrier (when
// non-empty) followed by the named fillers, and puts a "Mill <n>" sorcery in
// hand with {B} in the pool. Everything is arranged with logged events so
// replayCheck can rebuild the game from the log alone.
func millSnapshotBoard(t *testing.T, carrier string, fillers []string, n int) (*Engine, Config, state.ObjID, []state.ObjID) {
	t.Helper()
	spell := card(t, "Name:Grind\nManaCost:B\nTypes:Sorcery\nA:SP$ Mill | NumCards$ "+string(rune('0'+n))+" | Defined$ You\nOracle:x\n")
	deck := []*cards.Card{spell}
	names := []string{}
	if carrier != "" {
		deck = append(deck, corpusAlternativeCard(t, carrier))
		names = append(names, carrier)
	}
	for _, f := range fillers {
		deck = append(deck, card(t, "Name:"+f+"\nManaCost:no cost\nTypes:Sorcery\nOracle:x\n"))
		names = append(names, f)
	}
	deck = append(deck, mountainDeck(t, 40-len(deck))...)
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}, Tokens: testutil.CorpusRegistry(t).Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	find := func(name string) (state.ObjID, state.Zone) {
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
					return id, z
				}
			}
		}
		t.Fatalf("%s not in seat 0's hand or library", name)
		return 0, 0
	}
	spellID, z := find("Grind")
	if z == state.ZLibrary {
		e.emit(events.Event{Kind: events.MoveZone, Obj: spellID, From: state.ZLibrary, To: state.ZHand})
	}
	var top []state.ObjID
	for _, name := range names {
		id, z := find(name)
		if z == state.ZHand {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
		top = append(top, id)
	}
	newLib := slices.Clone(top)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if !slices.Contains(top, id) {
			newLib = append(newLib, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: newLib})
	return e, cfg, spellID, top
}

func millSnapshotCast(t *testing.T, e *Engine, spell state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "B")
	castFirstOf(t, e, spell)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(spell); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: the mill sorcery did not resolve (zone %s)", o.Zone)
	}
}

func graveNames(e *Engine) []string {
	var out []string
	for _, id := range e.G.Zone(state.ZGraveyard, 0) {
		out = append(out, e.G.Obj(id).Face().Name)
	}
	slices.Sort(out)
	return out
}

func milledIDs(e *Engine) []state.ObjID {
	var out []state.ObjID
	for _, ev := range e.L.Events {
		if events.IsMill(ev) {
			out = append(out, ev.Obj)
		}
	}
	return out
}

func testMillSnapshotCarrier(t *testing.T, carrier string) {
	t.Helper()
	e, cfg, spell, top := millSnapshotBoard(t, carrier, millSnapshotFillers, 3)
	if lib := e.G.Zone(state.ZLibrary, 0); len(lib) < 3 || !slices.Equal(lib[:len(top)], top) {
		t.Fatalf("precondition: library top = %v, want %v", lib, top)
	}
	millSnapshotCast(t, e, spell)
	if got, want := graveNames(e), []string{"Alpha", "Beta", "Grind"}; !slices.Equal(got, want) {
		t.Fatalf("graveyard = %v, want %v (the original top three, minus the replaced %s)", got, want, carrier)
	}
	if o := e.G.Obj(top[0]); o.Zone != state.ZLibrary {
		t.Fatalf("%s is in %s, want the library (its own replacement shuffled it back)", carrier, o.Zone)
	}
	replayCheck(t, e, cfg)
}

func TestMillSnapshotsTopNBeforeReplacements(t *testing.T) {
	t.Parallel()
	testMillSnapshotCarrier(t, "Darksteel Colossus")
}

func TestMillSnapshotProgenitus(t *testing.T) {
	t.Parallel()
	testMillSnapshotCarrier(t, "Progenitus")
}

func TestMillSnapshotPlainLibraryUnchanged(t *testing.T) {
	t.Parallel()
	e, cfg, spell, top := millSnapshotBoard(t, "", millSnapshotFillers, 3)
	millSnapshotCast(t, e, spell)
	if got := milledIDs(e); !slices.Equal(got, top[:3]) {
		t.Fatalf("milled ids = %v, want the top three in order %v", got, top[:3])
	}
	if got, want := graveNames(e), []string{"Alpha", "Beta", "Gamma", "Grind"}; !slices.Equal(got, want) {
		t.Fatalf("graveyard = %v, want %v", got, want)
	}
	replayCheck(t, e, cfg)
}

func TestMillSnapshotShortLibrary(t *testing.T) {
	t.Parallel()
	e, cfg, spell, top := millSnapshotBoard(t, "", millSnapshotFillers[:2], 3)
	for _, id := range e.G.Zone(state.ZLibrary, 0)[len(top):] {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZExile})
	}
	if lib := e.G.Zone(state.ZLibrary, 0); !slices.Equal(lib, top) {
		t.Fatalf("precondition: library = %v, want exactly %v", lib, top)
	}
	millSnapshotCast(t, e, spell)
	if got, want := graveNames(e), []string{"Alpha", "Beta", "Grind"}; !slices.Equal(got, want) {
		t.Fatalf("graveyard = %v, want %v", got, want)
	}
	if got := milledIDs(e); !slices.Equal(got, top) {
		t.Fatalf("milled ids = %v, want %v", got, top)
	}
	if n := len(e.G.Zone(state.ZLibrary, 0)); n != 0 {
		t.Fatalf("library = %d cards, want empty", n)
	}
	replayCheck(t, e, cfg)
}
