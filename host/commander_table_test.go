package host

// Task m39: making Commander reachable from the product. The engine has had
// rules.Format/StartingLife/Commanders since m30-m35, but a TableConfig
// could not say "commander" — every table gorged served was a 20-life
// constructed game and the command-zone panel m36 built rendered empty on
// every table that would ever exist. This file pins the host half: a
// commander table builds its matches with the format's rules.Config (40
// life, a command zone per seat from the deck's own commanders), replays
// them through the host's own replay build (R-8.4 — the sidecar must carry
// the format/life/commanders or a Commander match stops replaying), and
// rejects at the config boundary a commander table dealt a deck without a
// commander. The decks are the m38 interim Foundations decks, resolved
// through the same deck.File.CommanderIndex the product uses.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/view"
)

// commanderDeckNames is the five m38 interim mono-colour Commander decks,
// the smallest real set a commander table can deal (an invalid or
// commandless deck would make these tests about the fixture, not the host).
var commanderDeckNames = [...]string{
	"foundations-calling-all-angels",
	"foundations-keen-engineering",
	"foundations-reign-of-dragons",
	"foundations-tramplesaurus-rex",
	"foundations-wretched-ranks",
}

// commanderDeckLoader resolves the five interim decks through the compiled
// corpus, attaching each file's commander index exactly as gorged's loader
// does (deck.File.CommanderIndex, validated by deck.ValidateCommander), so
// the host gets its command zone from the same resolution the product uses
// and the tests cannot disagree with gorged about where a commander sits.
func commanderDeckLoader(t *testing.T) func(string) (Deck, error) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	byName := make(map[string]Deck, len(commanderDeckNames))
	for _, n := range commanderDeckNames {
		f := testutil.RepoDeckFile(t, n)
		d := Deck{Name: n, Cards: testutil.RepoDeck(t, reg, n)}
		if f.Commander != "" {
			d.Commanders = []int{f.CommanderIndex()}
		}
		byName[n] = d
	}
	return func(name string) (Deck, error) {
		d, ok := byName[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		return d, nil
	}
}

// commanderTable is a two-seat commander table: the smallest shape that has
// a command zone at all, seeded for a real game (m39 measured seeds
// 1000-1015 to produce games that reach a winner with commanders cast —
// see rules/commander_decks_test.go's repoCommanderGames — and 1001 is the
// keen-engineering-as-seat-0 evidence seed).
func commanderTable(id TableID, deck0, deck1 string) TableConfig {
	return TableConfig{ID: id, Name: "Table " + string(id), Seats: 2, Decks: []string{deck0, deck1},
		Seed: 1001, Pace: 0, Spectator: view.Omniscient, Perpetual: false, Format: FormatCommander}
}

// finishedCommanderMatch plays one commander table (keen-engineering vs
// tramplesaurus-rex, the seed-1001 evidence matchup) to completion and
// returns the match, its table and the live chain head, so the replay tests
// below drive the host's own rebuild. It takes a match slot for its
// registry, like every host test that builds one.
func finishedCommanderMatch(t *testing.T) (*Registry, *table, *match, string) {
	t.Helper()
	takeMatchSlot(t)
	r, err := New(Options{LoadDeck: commanderDeckLoader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(commanderTable("t1", commanderDeckNames[1], commanderDeckNames[3])); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	m := tb.history[0]
	tb.mu.RUnlock()
	if m == nil || m.state != protocol.MatchFinished {
		t.Fatalf("commander table did not finish: %+v", m)
	}
	return r, tb, m, m.e.L.Head()
}

// TestCommanderTableBuildsACommanderMatch is the format gate's host half: a
// TableConfig with FormatCommander builds its match with the format's
// rules.Config — the engine's FormatCommander, 40 life at genesis, and a
// command zone per seat from the deck's own commanders — so the m36
// command-zone projection is populated on a table that will actually exist.
// The observed surface is the projected View: at genesis every seat starts
// at 40 life with its commander in the zone, and at the head every seat's
// roster and its cast tally run parallel. Where the same decks on a
// constructed table project 20 life and empty zones, that contrast is the
// next test.
func TestCommanderTableBuildsACommanderMatch(t *testing.T) {
	r, tb, m, liveHead := finishedCommanderMatch(t)

	// The match's engine config is the format gate itself: FormatCommander
	// is what turns on the CR 903.8 tax, the CR 903.9 return and the CR
	// 903.10 clock — the commands the deck declares would otherwise be
	// inert pile cards.
	if m.cfg.Format != rules.FormatCommander {
		t.Fatalf("commander match built with format %v, want FormatCommander", m.cfg.Format)
	}

	v, err := r.ViewAt("t1", 1, uint64(len(m.e.L.Events)-1))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range v.Players {
		if len(p.Commanders) == 0 {
			t.Fatalf("seat %s projected no commanders roster on a commander table", p.Name)
		}
		if len(p.CommanderCasts) != len(p.Commanders) {
			t.Fatalf("seat %s casts %d parallel to %d commanders", p.Name, len(p.CommanderCasts), len(p.Commanders))
		}
	}

	// The opening life is the format's own: at genesis (seq 0, the first
	// Advance) every seat is at 40 with its commander in the zone — the
	// product evidence, wire-shaped.
	g, err := r.ViewAt("t1", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.Players {
		if p.Life != 40 {
			t.Fatalf("seat %s started at %d life, want 40 (CR 903.6)", p.Name, p.Life)
		}
		if len(p.Command) == 0 {
			t.Fatalf("seat %s started with an empty command zone on a commander table", p.Name)
		}
	}

	// The match's own replay, through the host's build, is the other half
	// of the same Config: a sidecar that lost the format would replay a
	// 20-life game and diverge immediately. Asserted once here so the
	// Config-under-test is the same one the view tests observed.
	sc := tb.cfg.Format // the table's format is what built the match
	if sc != FormatCommander {
		t.Fatalf("table format %v, want commander", sc)
	}
	sm, err := r.matchForLog(tb, m.sidecar(), m.e.L)
	if err != nil {
		t.Fatalf("commander match does not replay: %v", err)
	}
	if got, want := sm.e.L.Head(), liveHead; got != want {
		t.Fatalf("commander replay head %s, live head %s", got, want)
	}
}

// TestCommanderTableDefaultsLife40 is the 40-life default half: a commander
// table that never sets StartingLife (0, the zero value) must still play 40
// — the default is resolved by the host, once, at match build, so the
// sidecar records the life the match actually played with and a replay
// rebuilds it exactly. Killing this resolution is the mutation the task
// gates on: a commander table that silently plays 20 is the pre-m39
// disease.
func TestCommanderTableDefaultsLife40(t *testing.T) {
	r, _, m, _ := finishedCommanderMatch(t)
	if m.cfg.StartingLife != 40 {
		t.Fatalf("commander match config life %d, want 40 (CR 903.6 default)", m.cfg.StartingLife)
	}
	g, err := r.ViewAt("t1", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := g.Players[0].Life; got != 40 {
		t.Fatalf("commander table with StartingLife 0 played at %d, want 40", got)
	}
	if sc := m.sidecar(); sc.StartingLife != 40 {
		t.Fatalf("sidecar recorded life %d, want 40 (the value the replay reads back)", sc.StartingLife)
	}
}

// TestConstructedTableStays20Life pins the format gate's other half: the
// same decks on a table whose Format is the zero value are a 20-life game
// with no command zone — a commander table's commands cannot leak onto a
// constructed table, and a constructed table cannot accidentally become a
// Commander game. This is what keeps the default gorged table byte-
// identical to every pre-m39 table, and the m39 default -format
// constructed is exactly this TableConfig.
func TestConstructedTableStays20Life(t *testing.T) {
	takeMatchSlot(t)
	r, err := New(Options{LoadDeck: commanderDeckLoader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	c := commanderTable("t1", commanderDeckNames[1], commanderDeckNames[3])
	c.Format = FormatConstructed // the zero value — what an old tables.json loads as
	if err := r.AddTable(c); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	m := tb.history[0]
	tb.mu.RUnlock()
	if m == nil || m.state != protocol.MatchFinished {
		t.Fatalf("constructed table did not finish: %+v", m)
	}
	if m.cfg.Format != 0 {
		t.Fatalf("constructed match config format %v, want 0", m.cfg.Format)
	}
	if m.cfg.StartingLife != 0 {
		t.Fatalf("constructed match config life %d, want 0 (engine's 20)", m.cfg.StartingLife)
	}
	if len(m.cfg.Commanders) != 0 {
		t.Fatalf("constructed match carries commander indices %v, want none", m.cfg.Commanders)
	}
	v, err := r.ViewAt("t1", 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range v.Players {
		if p.Life != 20 {
			t.Fatalf("constructed seat %s started at %d life, want 20", p.Name, p.Life)
		}
		if len(p.Commanders) != 0 || len(p.Command) != 0 {
			t.Fatalf("constructed seat %s has command zone %v/%v, want none", p.Name, p.Command, p.Commanders)
		}
	}
}

// TestCommanderMatchReplaysExactly is R-8.4 for the format, asserted
// directly: the match replays from its log through the host's own replay
// build (matchForLog, host/viewat.go — a Config rebuilt from the sidecar
// alone). A sidecar that dropped the format, life or commanders would
// re-run the game as a 20-life constructed match: genesis is different,
// the first logged commander cast is refused against a game with no
// command zone, and Replay errors right here. The sidecar is asserted
// explicitly too: it is the value the replay must read back through, and
// the one site where a dropped write silently reads as 0.
func TestCommanderMatchReplaysExactly(t *testing.T) {
	takeMatchSlot(t)
	r, err := New(Options{LoadDeck: commanderDeckLoader(t), Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(commanderTable("t1", commanderDeckNames[1], commanderDeckNames[3])); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	m := tb.history[0]
	tb.mu.RUnlock()
	if m == nil || m.state != protocol.MatchFinished {
		t.Fatalf("commander table did not finish: %+v", m)
	}
	liveHead := m.e.L.Head()

	sc := m.sidecar()
	if sc.Format != FormatCommander {
		t.Fatalf("sidecar lost the format: %v, want commander", sc.Format)
	}
	if sc.StartingLife != 40 || len(sc.Commanders) != 2 {
		t.Fatalf("sidecar lost the commander config: life %d, %d commander rows, want 40 and 2", sc.StartingLife, len(sc.Commanders))
	}
	for i, cmds := range sc.Commanders {
		if len(cmds) != 1 {
			t.Fatalf("sidecar seat %d carries %d commander indices, want 1", i, len(cmds))
		}
	}

	sm, err := r.matchForLog(tb, sc, m.e.L)
	if err != nil {
		t.Fatalf("commander match does not replay: %v", err)
	}
	if got, want := sm.e.L.Head(), liveHead; got != want {
		t.Fatalf("commander replay head %s, live head %s", got, want)
	}

	// And the old-sidecar shape still loads (R-E5-2): a sidecar written
	// before m39 has no format/life/commanders keys and must read as Format
	// zero (constructed), life 0 (the engine's 20) and no commanders — the
	// values those matches played with — so an on-disk history predating
	// the task keeps serving untouched.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "t1"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := `{
  "table": "t1",
  "match": 1,
  "seed": 7,
  "names": ["a", "b"],
  "decks": ["a", "b"],
  "spectator": "omniscient",
  "state": "finished",
  "result": "win",
  "events": 431,
  "turns": 12
}
`
	if err := os.WriteFile(filepath.Join(dir, "t1", "1.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	sc2, err := readSidecar(dir, "t1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if sc2.Format != FormatConstructed || sc2.StartingLife != 0 || len(sc2.Commanders) != 0 {
		t.Fatalf("old sidecar loaded format/life/commanders %v/%d/%v, want zero/zero/none", sc2.Format, sc2.StartingLife, sc2.Commanders)
	}
}

// TestCommanderFormatSurvivesARestart is the persisted-table round-trip for
// the new TableConfig fields: a commander table's format is written verbatim
// into tables.json and read back by a fresh registry (MarshalText/
// UnmarshalText — the same on-the-wire name a person reads), and the match
// a restart serves from disk still replays. A sidecar or tables.json that
// lost the format would re-run a Commander match as a 20-life constructed
// game, and Events (which forces the replay) would error here.
func TestCommanderFormatSurvivesARestart(t *testing.T) {
	takeMatchSlot(t)
	dir := t.TempDir()
	o := diskOptions(t, dir)
	o.LoadDeck = commanderDeckLoader(t)
	var liveHead string
	var liveEvents int
	{
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		if err := r.AddTable(commanderTable("t1", commanderDeckNames[1], commanderDeckNames[3])); err != nil {
			t.Fatal(err)
		}
		_ = r.Start("t1")
		r.Wait("t1")
		ms, err := r.Matches("t1")
		if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
			t.Fatalf("matches before restart: %+v, %v", ms, err)
		}
		liveHead, liveEvents = ms[0].Head, ms[0].Events
		raw, err := os.ReadFile(filepath.Join(dir, "tables.json"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `"format": "commander"`) {
			t.Fatalf("tables.json does not record the table's format:\n%s", raw)
		}
		if err := r.Close(); err != nil {
			t.Fatal(err)
		}
	}

	r2, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r2.Close()
	ms, err := r2.Matches("t1")
	if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("after restart: %+v, %v", ms, err)
	}
	if ms[0].Head != liveHead || ms[0].Events != liveEvents {
		t.Fatalf("after restart: head %s/%d events, live was %s/%d", ms[0].Head, ms[0].Events, liveHead, liveEvents)
	}
	evs, err := r2.Events("t1", 1, 0)
	if err != nil {
		t.Fatalf("restarted commander match does not replay: %v", err)
	}
	if len(evs) != liveEvents {
		t.Fatalf("restarted replay serves %d events, want %d", len(evs), liveEvents)
	}
}

// TestCommanderTableRejectsADeckWithoutACommander is the config-boundary
// enforcement: a commander table whose loader reports a deck without
// commanders is refused by validate, with the table and deck named, before
// any match can build with a seat that plays without a command zone. This
// is the host's half of the startup validation (gorged validates the deck
// files themselves up front); between them a half-started commander table
// is impossible rather than merely unlikely.
func TestCommanderTableRejectsADeckWithoutACommander(t *testing.T) {
	takeMatchSlot(t)
	r, err := New(Options{LoadDeck: func(name string) (Deck, error) {
		// Every deck this loader names resolves to a commanderless deck:
		// the "a commander table dealt a constructed deck" shape.
		return Deck{Name: name, Cards: testutil.RepoDeck(t, testutil.CorpusRegistry(t), commanderDeckNames[1])}, nil
	}, Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := TableConfig{ID: "t1", Name: "Table 1", Seats: 2, Decks: []string{"a", "b"},
		Seed: 7, Pace: 0, Spectator: view.Omniscient, Perpetual: false, Format: FormatCommander}
	err = r.AddTable(c)
	if err == nil {
		t.Fatal("commander table with a commanderless deck accepted")
	}
	if !strings.Contains(err.Error(), "names no commander") {
		t.Fatalf("rejection does not name the gap: %v", err)
	}
}

// TestCommanderTableRejectsOutOfRangeCommanderIndex is the index-range half
// of the same boundary: the engine degrades an out-of-range commander index
// silently (the command-zone object is skipped), which a hash-chained game
// must never absorb from a config error — validate refuses the table, with
// the index named.
func TestCommanderTableRejectsOutOfRangeCommanderIndex(t *testing.T) {
	takeMatchSlot(t)
	loader := commanderDeckLoader(t)
	r, err := New(Options{LoadDeck: func(name string) (Deck, error) {
		d, derr := loader(name)
		if derr == nil && d.Commanders != nil {
			d.Commanders = []int{9999}
		}
		return d, derr
	}, Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	c := commanderTable("t1", commanderDeckNames[1], commanderDeckNames[3])
	err = r.AddTable(c)
	if err == nil {
		t.Fatal("out-of-range commander index accepted")
	}
	if !strings.Contains(err.Error(), "9999") {
		t.Fatalf("rejection does not name the index: %v", err)
	}
}
