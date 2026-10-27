package rules

import (
	"fmt"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g152: Manifest's Defined$ object forms (Remembered, ValidLibrary,
// ValidHand) and a declared SVar Amount$ (Forge ManifestBaseEffect).

// castOption submits seat 0's cast option for id.
func submitCastFor(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no cast option for %d: %+v", id, d.Options)
}

// manifestMoves returns the MoveZone events that manifest-entered a card.
func manifestMoves(e *Engine, from int) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.MoveZone && ev.To == state.ZBattlefield && ev.Counter == events.ManifestEntryCounter {
			out = append(out, ev)
		}
	}
	return out
}

// assertManifested checks the card is a face-down manifested permanent of seat p.
func assertManifested(t *testing.T, e *Engine, id state.ObjID, p state.PlayerID) {
	t.Helper()
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || !o.FaceDown || !o.Manifested {
		t.Fatalf("card %d: zone %s FaceDown %v Manifested %v, want a manifested permanent", id, o.Zone, o.FaceDown, o.Manifested)
	}
	if o.Controller != p {
		t.Fatalf("card %d controller = %d, want %d", id, o.Controller, p)
	}
}

func TestWriteIntoBeingManifestsTheChosenCard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Write into Being")
	id := searchMoveByName(t, e, "Write into Being", state.ZHand)
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) < 3 {
		t.Fatalf("library has %d cards", len(lib))
	}
	first, second := lib[0], lib[1]
	addMana(t, e, 0, "CCU")
	submitCastFor(t, e, id)
	cutoff := len(e.L.Events)
	d := passUntilAskKind(t, e, decision.KChoose, 20)
	pick := -1
	for _, o := range d.Options {
		if o.Obj == second {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("the Dig did not offer the second card %d: %+v", second, d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 30)

	if mv := manifestMoves(e, cutoff); len(mv) != 1 || mv[0].Obj != second {
		t.Fatalf("manifest moves = %+v, want exactly the chosen card %d", mv, second)
	}
	assertManifested(t, e, second, 0)
	if got := e.G.Obj(first); got.Zone != state.ZLibrary {
		t.Fatalf("the unchosen card is in %s, want the library", got.Zone)
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}

func TestGhastlyConscriptionManifestsTheExiledCreatures(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := manifestEngine(t, reg, "Ghastly Conscription")
	var bears []state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 1) {
			if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == "Grizzly Bears" && len(bears) < 2 {
				bears = append(bears, id)
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZGraveyard})
			}
		}
	}
	if len(bears) != 2 {
		t.Fatalf("staged %d bears, want 2", len(bears))
	}
	for _, id := range bears {
		if e.G.Obj(id).Zone != state.ZGraveyard {
			t.Fatalf("bear %d is not in the graveyard", id)
		}
	}
	id := searchMoveByName(t, e, "Ghastly Conscription", state.ZHand)
	addMana(t, e, 0, "CCCCCBB")
	submitCastFor(t, e, id)
	cutoff := len(e.L.Events)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after casting: %+v, want the player target ask", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == 0 && o.Player == 1 {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("seat 1 not offered as a target: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 30)

	for _, b := range bears {
		assertManifested(t, e, b, 0)
	}
	if mv := manifestMoves(e, cutoff); len(mv) != 2 {
		t.Fatalf("%d manifest moves, want 2: %+v", len(mv), mv)
	}
	for _, ev := range e.L.Events[cutoff:] {
		if ev.Kind == events.Shuffle {
			t.Fatalf("a Shuffle event was emitted: %+v", ev)
		}
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}

// castFixtureInstant casts the fixture instant (free) and drains the stack.
func castFixtureInstant(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "C")
	submitCastFor(t, e, id)
	passUntilStackEmpty(t, e, 20)
}

func TestManifestValidLibraryTakesTheNamedTopCard(t *testing.T) {
	t.Parallel()
	src := "Name:Named Top\nManaCost:0\nTypes:Instant\n" +
		"A:SP$ Manifest | Amount$ 1 | Defined$ ValidLibrary Card.TopLibrary+ActivePlayerCtrl\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 4402, src)
	if e.G.Active != 0 {
		t.Fatalf("active player = %d, want seat 0's turn", e.G.Active)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	top, next := lib[0], lib[1]
	oppTop := e.G.Zone(state.ZLibrary, 1)[0]
	cutoff := len(e.L.Events)
	castFixtureInstant(t, e, id)
	if mv := manifestMoves(e, cutoff); len(mv) != 1 || mv[0].Obj != top {
		t.Fatalf("manifest moves = %+v, want seat 0's top card %d", mv, top)
	}
	assertManifested(t, e, top, 0)
	if e.G.Obj(next).Zone != state.ZLibrary || e.G.Obj(oppTop).Zone != state.ZLibrary {
		t.Fatal("a card other than the named top card left a library")
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}

func TestManifestAmountReadsDeclaredSVar(t *testing.T) {
	t.Parallel()
	src := "Name:Declared Two\nManaCost:0\nTypes:Instant\n" +
		"A:SP$ Manifest | Amount$ Y\nSVar:Y:Number$2\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 4403, src)
	lib := e.G.Zone(state.ZLibrary, 0)
	a, b, c := lib[0], lib[1], lib[2]
	cutoff := len(e.L.Events)
	castFixtureInstant(t, e, id)
	mv := manifestMoves(e, cutoff)
	if len(mv) != 2 || mv[0].Obj != a || mv[1].Obj != b {
		t.Fatalf("manifest moves = %+v, want the top two cards %d, %d one at a time", mv, a, b)
	}
	assertManifested(t, e, a, 0)
	assertManifested(t, e, b, 0)
	if e.G.Obj(c).Zone != state.ZLibrary {
		t.Fatal("a third card was manifested")
	}
	noUnimplementedManifest(t, e)
	replayCheck(t, e, cfg)
}

// TestManifestFamilyCorpusShapesAreSupported runs every Manifest,
// ManifestDread and Cloak SA in the corpus through the shared shape check the
// primitives call: the only loud shapes left have no carrier. It also pins
// that the sweep sees the carriers the ticket closed.
func TestManifestFamilyCorpusShapesAreSupported(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	seen := map[string]int{}
	badSet := map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			walkAllSAs(f, func(sa *cards.SA) {
				switch sa.API {
				case "Manifest", "ManifestDread", "Cloak":
				default:
					return
				}
				seen[sa.API]++
				if sa.API == "Manifest" && sa.Params["Defined"] == "Remembered" {
					seen["Manifest Remembered"]++
				}
				if !effects.ManifestFamilyShapeSupported(sa.API, sa, func(name string) bool { _, ok := f.SVars[name]; return ok }) {
					badSet[fmt.Sprintf("%s: %s", f.Name, sa.Line)] = true
				}
			})
		}
	}
	bad := make([]string, 0, len(badSet))
	for b := range badSet {
		bad = append(bad, b)
	}
	sort.Strings(bad)
	for _, b := range bad {
		t.Errorf("corpus shape is still loud: %s", b)
	}
	for _, api := range []string{"Manifest", "ManifestDread", "Cloak", "Manifest Remembered"} {
		if seen[api] == 0 {
			t.Fatalf("the sweep saw no %s SA: the census is vacuous", api)
		}
	}
}
