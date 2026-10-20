package matchrecord_test

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/matchrecord"
)

// fixtureRegistry is a corpus-free registry: a vanilla creature and a
// basic-typed land, enough for a game that runs to a decking.
func fixtureRegistry(t *testing.T) *cards.Registry {
	t.Helper()
	reg := cards.NewRegistry()
	for _, src := range []string{
		"Name:Test Pipewhale\nManaCost:1\nTypes:Creature Whale\nPT:2/2\nOracle:x\n",
		"Name:Test Wastes\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nOracle:x\n",
	} {
		c, diags := cards.ParseBytes("fixture.txt", []byte(src))
		if len(diags) != 0 {
			t.Fatalf("fixture card: %v", diags)
		}
		if d := c.Link(); len(d) != 0 {
			t.Fatalf("fixture link: %v", d)
		}
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		reg.Add(c)
	}
	return reg
}

func fixtureDeck(t *testing.T, reg *cards.Registry) []*cards.Card {
	t.Helper()
	whale, _ := reg.Lookup("Test Pipewhale")
	wastes, _ := reg.Lookup("Test Wastes")
	d := make([]*cards.Card, 0, 30)
	for i := 0; i < 15; i++ {
		d = append(d, whale, wastes)
	}
	return d
}

// playFixture plays up to n intents of a fixture game with heuristic bots
// and returns the config and engine.
func playFixture(t *testing.T, reg *cards.Registry, n int) (rules.Config, *rules.Engine) {
	t.Helper()
	cfg := rules.Config{
		Seed: 99, Names: []string{"a", "b"}, PlayerNames: []string{"Ann", "Bo"},
		Decks: [][]*cards.Card{fixtureDeck(t, reg), fixtureDeck(t, reg)}, Mulligans: 1,
	}
	e := rules.New(cfg)
	e.Advance()
	bots := []*heuristic.Bot{heuristic.NewBot(1), heuristic.NewBot(2)}
	for i := 0; i < n && !e.G.Over && e.Pending() != nil; i++ {
		d := e.Pending()
		in, err := bots[d.Player].Decide(context.Background(), view.Project(e.G, e, d.Player, d), *d)
		if err != nil {
			t.Fatal(err)
		}
		if err := e.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	return cfg, e
}

func TestWriteLoadVerify(t *testing.T) {
	reg := fixtureRegistry(t)
	cfg, e := playFixture(t, reg, 400)
	if len(e.L.Intents) < 20 {
		t.Fatalf("fixture game logged only %d intents", len(e.L.Intents))
	}
	p := matchrecord.Producer{Tool: "unit", GorgeCommit: "abc", ForgeRef: "def", Policies: []string{"heuristic", "heuristic"}, RunID: "r"}
	rec, err := matchrecord.New(cfg, e.L, p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "x"+matchrecord.Ext)
	if err := matchrecord.Write(path, rec); err != nil {
		t.Fatal(err)
	}
	back, rcfg, err := matchrecord.Load(path, reg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.Producer, p) || back.Format != matchrecord.FormatV1 || back.Head != e.L.Head() {
		t.Fatalf("round trip: %+v head %s", back.Producer, back.Head)
	}
	if err := matchrecord.Verify(back, rcfg); err != nil {
		t.Fatal(err)
	}

	// A corrupted head field is caught before any replay.
	bad := *back
	bad.Head = "0000000000000000"
	if err := matchrecord.Verify(&bad, rcfg); err == nil {
		t.Fatal("Verify accepted a tampered head")
	}
	// A tampered event (with its head recomputed, so only the replay can
	// see it) is a located divergence.
	tl := &events.Log{Seed: back.Log.Seed, Events: append([]events.Event(nil), back.Log.Events...), Intents: back.Log.Intents}
	k := len(tl.Events) / 2
	tl.Events[k].Amount += 7
	bad = *back
	bad.Log, bad.Head = tl, tl.HeadAt(len(tl.Events))
	err = matchrecord.Verify(&bad, rcfg)
	var div *replay.Divergence
	if !errors.As(err, &div) || div.Seq != uint64(k) {
		t.Fatalf("tampered event %d: %v", k, err)
	}
	// A chance transcript is not ordinary replay input.
	bad = *back
	bad.Chance = []rules.ChanceDraw{{Bound: 2, Value: 1}}
	if err := matchrecord.Verify(&bad, rcfg); err == nil {
		t.Fatal("Verify accepted a chance transcript")
	}
}

func TestReadRefusesAnotherFormat(t *testing.T) {
	reg := fixtureRegistry(t)
	cfg, e := playFixture(t, reg, 5)
	rec, err := matchrecord.New(cfg, e.L, matchrecord.Producer{})
	if err != nil {
		t.Fatal(err)
	}
	rec.Format = "gorge-match-0"
	path := filepath.Join(t.TempDir(), "x"+matchrecord.Ext)
	if err := matchrecord.Write(path, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := matchrecord.Read(path); err == nil || !strings.Contains(err.Error(), "format") {
		t.Fatalf("Read: %v", err)
	}
}

func TestNewRefusesAnUnhashedLog(t *testing.T) {
	reg := fixtureRegistry(t)
	cfg, _ := playFixture(t, reg, 0)
	if _, err := matchrecord.New(cfg, &events.Log{NoHash: true}, matchrecord.Producer{}); err == nil {
		t.Fatal("New accepted a NoHash log")
	}
}

// TestMatchConfigRoundTrip pins every replay-input field through MatchOf,
// JSON and Config: the shared builder feedback.Load also goes through.
func TestMatchConfigRoundTrip(t *testing.T) {
	reg := fixtureRegistry(t)
	whale, _ := reg.Lookup("Test Pipewhale")
	chooser := 1
	cfg := rules.Config{
		Seed: 5, Names: []string{"a", "b"}, PlayerNames: []string{"Ann", "Bo"},
		Decks:       [][]*cards.Card{fixtureDeck(t, reg), fixtureDeck(t, reg)},
		Sideboards:  [][]*cards.Card{{whale}, nil},
		PlanarDecks: [][]*cards.Card{{whale, whale}, {whale}},
		Format:      rules.FormatCommander, StartingLife: 40, Commanders: [][]int{{0}, {0}},
		Mulligans: 2, StartChooser: &chooser, NameUniverse: reg.Universe(),
	}
	m, err := matchrecord.MatchOf(cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var m2 matchrecord.Match
	if err := json.Unmarshal(raw, &m2); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(m, m2) {
		t.Fatalf("JSON round trip:\n%+v\n%+v", m, m2)
	}
	tokens := map[string]*cards.Card{"t": whale}
	got, err := m2.Config(reg, tokens)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format != cfg.Format || got.StartingLife != 40 || !reflect.DeepEqual(got.Commanders, cfg.Commanders) ||
		got.Mulligans != 2 || got.StartChooser == nil || *got.StartChooser != 1 || got.NameUniverse == nil ||
		got.NameUniverseNames != nil || got.DisablePaidManaCensus || got.Tokens["t"] != whale ||
		!reflect.DeepEqual(got.PlayerNames, cfg.PlayerNames) || !reflect.DeepEqual(got.Names, cfg.Names) {
		t.Fatalf("round trip lost a field: %+v", got)
	}
	for i := range cfg.Decks {
		if !reflect.DeepEqual(got.Decks[i], cfg.Decks[i]) || !reflect.DeepEqual(got.PlanarDecks[i], cfg.PlanarDecks[i]) {
			t.Fatalf("seat %d decks differ", i)
		}
	}
	if len(got.Sideboards) != 2 || len(got.Sideboards[0]) != 1 || len(got.Sideboards[1]) != 0 {
		t.Fatalf("sideboards %v", got.Sideboards)
	}
	// The chooser is copied, never aliased into the record.
	chooser = 0
	if *m.StartChooser != 1 {
		t.Fatal("MatchOf aliased the StartChooser pointer")
	}

	cfg.DisablePaidManaCensus = true
	cfg.NameUniverse = nil
	cfg.Format = rules.FormatConstructed
	m, _ = matchrecord.MatchOf(cfg)
	got, _ = m.Config(reg, nil)
	if !got.DisablePaidManaCensus || got.NameUniverse != nil || got.Format != rules.FormatConstructed {
		t.Fatalf("mode bits: %+v", got)
	}

	m.DeckCards = [][]string{{"Card The Corpus Never Knew"}}
	if _, err := m.Config(reg, nil); err == nil || !strings.Contains(err.Error(), "seat 0 card 0") {
		t.Fatalf("unknown card: %v", err)
	}
	m.DeckCards = [][]string{{"Test Pipewhale"}}
	m.Format = "vintage"
	if _, err := m.Config(reg, nil); err == nil {
		t.Fatal("Config accepted an unknown format")
	}
}
