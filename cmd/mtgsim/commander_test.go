package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

var commanderPair = []string{"foundations-tramplesaurus-rex", "foundations-wretched-ranks"}

// TestCommanderConfigSeatsTheCommanders: a commander gameConfig starts both
// seats at 40 life with each commander in the command zone, none in a library.
func TestCommanderConfigSeatsTheCommanders(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][]*cards.Card, len(commanderPair))
	cmds := make([][]int, len(commanderPair))
	for i, n := range commanderPair {
		if decks[i], err = testutil.LoadRepoDeck(reg, n); err != nil {
			t.Fatal(err)
		}
		cmds[i] = testutil.RepoDeckFile(t, n).CommanderIndices()
		if len(cmds[i]) == 0 {
			t.Fatalf("deck %s: no commander indices", n)
		}
	}
	cfg := gameConfig(0, commanderPair, decks, reg, rules.FormatCommander, cmds, 0)
	e := rules.New(cfg)
	e.Advance()
	if e.Pending() == nil {
		t.Fatal("no first decision")
	}
	for i := range commanderPair {
		pl := e.G.Players[i]
		if pl.Life != 40 {
			t.Errorf("seat %d: life %d, want 40", i, pl.Life)
		}
		if len(pl.Commanders) == 0 {
			t.Fatalf("seat %d: no commanders", i)
		}
		lib := map[state.ObjID]bool{}
		for _, id := range e.G.Zone(state.ZLibrary, state.PlayerID(i)) {
			lib[id] = true
		}
		for _, id := range pl.Commanders {
			if z := e.G.Obj(id).Zone; z != state.ZCommand {
				t.Errorf("seat %d: commander %d in zone %v, want command", i, id, z)
			}
			if lib[id] {
				t.Errorf("seat %d: commander %d is in the library", i, id)
			}
		}
	}
}

// TestCommanderRunIsDeterministic: two identical commander runs print the
// same bytes, and the game finishes and replays.
func TestCommanderRunIsDeterministic(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	o := simOptions{Decks: strings.Join(commanderPair, ","), Seats: 2, Games: 1, Verify: true, Dir: dir, Format: "commander"}
	var a, b bytes.Buffer
	if err := runSim(o, &a); err != nil {
		t.Fatalf("first run: %v\n%s", err, a.String())
	}
	if err := runSim(o, &b); err != nil {
		t.Fatalf("second run: %v\n%s", err, b.String())
	}
	out := a.String()
	if out != b.String() {
		t.Errorf("runs differ:\n%s\n---\n%s", out, b.String())
	}
	for _, want := range []string{"2 seats,", "replay OK"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
	if !strings.Contains(out, "winner=") && !strings.Contains(out, "draw") {
		t.Errorf("output reports no finished game: %q", out)
	}
}

// TestCommanderFormatRefusesAConstructedDeck: a deck with no commander, an
// unknown format and a one-seat commander run are all errors.
func TestCommanderFormatRefusesAConstructedDeck(t *testing.T) {
	dir := testutil.CorpusDirOrSkip(t)
	var buf bytes.Buffer
	err := runSim(simOptions{Decks: "uw-tempo,foundations-wretched-ranks", Seats: 2, Games: 1, Dir: dir, Format: "commander"}, &buf)
	if err == nil || !strings.Contains(err.Error(), `deck "uw-tempo" names no commander`) {
		t.Errorf("constructed deck under commander: err = %v", err)
	}
	err = runSim(simOptions{Seats: 2, Games: 1, Dir: dir, Format: "modern"}, &buf)
	if err == nil || !strings.Contains(err.Error(), "constructed") || !strings.Contains(err.Error(), "commander") {
		t.Errorf("unknown format: err = %v", err)
	}
	err = runSim(simOptions{Decks: "foundations-wretched-ranks", Seats: 1, Games: 1, Dir: dir, Format: "commander"}, &buf)
	if err == nil || !strings.Contains(err.Error(), "-format commander needs at least 2 seats") {
		t.Errorf("one-seat commander: err = %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("a refused run printed output: %q", buf.String())
	}
}

// TestCommanderDefaultDecks: the commander default is the first sorted
// Commander decks; the constructed default is still the Legacy decks.
func TestCommanderDefaultDecks(t *testing.T) {
	var want []string
	for _, n := range testutil.RepoDeckNames() {
		if len(testutil.RepoDeckFile(t, n).CommanderNames()) > 0 {
			want = append(want, n)
		}
	}
	if len(want) < 4 {
		t.Fatalf("only %d Commander repo decks", len(want))
	}
	got, err := deckNames("", 4, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != strings.Join(want[:4], ",") {
		t.Errorf("commander default = %v, want %v", got, want[:4])
	}
	for _, n := range got {
		if len(testutil.RepoDeckFile(t, n).CommanderNames()) == 0 {
			t.Errorf("default commander deck %s names no commander", n)
		}
	}
	got, err = deckNames("", 2, false)
	if err != nil {
		t.Fatal(err)
	}
	legacy := testutil.LegacyDeckNames()
	if strings.Join(got, ",") != strings.Join(legacy[:2], ",") {
		t.Errorf("constructed default = %v, want %v", got, legacy[:2])
	}
}
