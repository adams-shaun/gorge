package rules

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The set-spread chain-head goldens: a second, wider family beside TestHeads.
// TestHeads pins four games over 12 Legacy decks; this family pins a few dozen
// small games whose decks are drawn from the Standard program's sets
// (testdata/sets/<SET>.txt), chosen so that between them they play the card
// features -- keywords, ability APIs, trigger modes, static modes, replacement
// events and sets -- that the acceptance games leave unplayed. The games are
// pinned data, not derived: testdata/setheads/games.txt holds each game's seed
// and every deck's card list, so a corpus bump or a new card never changes what
// a golden plays, and testdata/setheads/heads/<id>.txt holds one head per game
// (a file per game, so two branches re-pinning different games do not
// conflict). Regenerate the game list with setheads_generate_test.go; re-pin
// heads with GORGE_UPDATE_SETHEADS=1.
const (
	setHeadsDir       = "testdata/setheads"
	updateSetHeadsEnv = "GORGE_UPDATE_SETHEADS"
	dumpSetHeadsEnv   = "GORGE_DUMP_SETHEADS"
)

type setDeckEntry struct {
	Count int
	Name  string
}

type setDeck struct {
	Label string
	Cards []setDeckEntry
}

type setGame struct {
	ID    string
	Seed  uint64
	Decks []setDeck
}

func setHeadPath(id string) string {
	return filepath.Join(setHeadsDir, "heads", id+".txt")
}

// parseSetGames reads games.txt:
//
//	game <id> <seed>
//	deck <label>
//	<count> <card name>
//
// Blank lines and lines starting with # are ignored. A deck belongs to the game
// above it; the order of decks is the seating order.
func parseSetGames(path string) ([]setGame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var games []setGame
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		head, rest, _ := strings.Cut(line, " ")
		switch head {
		case "game":
			id, seed, ok := strings.Cut(rest, " ")
			s, err := strconv.ParseUint(seed, 10, 64)
			if !ok || err != nil {
				return nil, fmt.Errorf("%s:%d: want `game <id> <seed>`", path, n)
			}
			games = append(games, setGame{ID: id, Seed: s})
		case "deck":
			if len(games) == 0 {
				return nil, fmt.Errorf("%s:%d: deck before any game", path, n)
			}
			g := &games[len(games)-1]
			g.Decks = append(g.Decks, setDeck{Label: rest})
		default:
			c, err := strconv.Atoi(head)
			if err != nil || len(games) == 0 || len(games[len(games)-1].Decks) == 0 {
				return nil, fmt.Errorf("%s:%d: want `<count> <card>`, got %q", path, n, line)
			}
			g := &games[len(games)-1]
			d := &g.Decks[len(g.Decks)-1]
			d.Cards = append(d.Cards, setDeckEntry{Count: c, Name: rest})
		}
	}
	return games, sc.Err()
}

// resolveSetGame seats g's decks through reg.
func resolveSetGame(reg *cards.Registry, g setGame) (Config, error) {
	names := make([]string, len(g.Decks))
	decks := make([][]*cards.Card, len(g.Decks))
	for i, d := range g.Decks {
		names[i] = d.Label
		for _, ent := range d.Cards {
			c, ok := reg.Lookup(ent.Name)
			if !ok {
				return Config{}, fmt.Errorf("game %s deck %s: card %q is not in the corpus", g.ID, d.Label, ent.Name)
			}
			for k := 0; k < ent.Count; k++ {
				decks[i] = append(decks[i], c)
			}
		}
	}
	cfg := AcceptanceConfig(reg, names, decks)
	cfg.Seed = g.Seed
	return cfg, nil
}

// playSetGame plays g the acceptance way, replays it from its own log, and
// returns the finished engine. The replay agreeing with the live chain is the
// same determinism check playAcceptance makes.
func playSetGame(t testing.TB, reg *cards.Registry, g setGame) *Engine {
	t.Helper()
	cfg, err := resolveSetGame(reg, g)
	if err != nil {
		t.Fatal(err)
	}
	e, _, err := PlayAcceptance(cfg, acceptanceTestBot, nil)
	if err != nil {
		t.Fatalf("game %s: %v", g.ID, err)
	}
	re, err := replayFor(cfg, e.L)
	if err != nil {
		t.Fatalf("game %s: %v", g.ID, err)
	}
	if re.L.Head() != e.L.Head() {
		t.Fatalf("game %s: chain %s, replay %s", g.ID, e.L.Head(), re.L.Head())
	}
	return e
}

func describeSetGame(g setGame) string {
	labels := make([]string, len(g.Decks))
	for i, d := range g.Decks {
		labels[i] = d.Label
	}
	return fmt.Sprintf("%d seats [%s], seed %d", len(g.Decks), strings.Join(labels, ", "), g.Seed)
}

const setHeadsRePin = updateSetHeadsEnv + "=1 go test ./engine/rules/ -run '^TestSetHeads$'"

// TestSetHeads pins the chain head of every game in testdata/setheads/games.txt.
func TestSetHeads(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("long")
	}
	reg := testutil.CorpusRegistry(t)
	games, err := parseSetGames(filepath.Join(setHeadsDir, "games.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(games) == 0 {
		t.Fatal("testdata/setheads/games.txt lists no game")
	}
	update := os.Getenv(updateSetHeadsEnv) == "1"
	dump := os.Getenv(dumpSetHeadsEnv)
	for _, g := range games {
		t.Run(g.ID, func(t *testing.T) {
			t.Parallel()
			e := playSetGame(t, reg, g)
			got := e.L.Head()
			if dump != "" {
				dumpEvents(t, filepath.Join(dump, g.ID+".events"), e)
			}
			if update {
				if err := os.WriteFile(setHeadPath(g.ID), []byte(got+"\n"), 0o644); err != nil {
					t.Fatalf("game %s: %v", g.ID, err)
				}
				return
			}
			raw, err := os.ReadFile(setHeadPath(g.ID))
			if err != nil {
				t.Fatalf("game %s: reading the pinned head: %v (re-pin with %s)", g.ID, err, setHeadsRePin)
			}
			if want := strings.TrimSpace(string(raw)); got != want {
				t.Errorf("game %s (%s): chain head %s, golden %s, %d events, turn %d -- if this move is intended, "+
					"re-pin with `%s` and name the first diverging event and its cause in the COMMIT MESSAGE (not in a file). "+
					"Find the event: run this test with %s=<dir> on the base and on this tree, then `diff` the two "+
					"%s/%s.events files (one JSON event per line; the first differing line is the first diverging event)",
					g.ID, describeSetGame(g), got, want, len(e.L.Events), e.G.Turn, setHeadsRePin,
					dumpSetHeadsEnv, "<dir>", g.ID)
			}
		})
	}
}

// dumpEvents writes e's event stream, one JSON event per line.
func dumpEvents(t testing.TB, path string, e *Engine) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, ev := range e.L.Events {
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(b)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
}
