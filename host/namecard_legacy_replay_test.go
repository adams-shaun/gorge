package host

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// TestLegacyETBNameCardsReplayWithoutUniverse is an actual pre-feature replay
// configuration: no NameUniverse reaches either the live engine or
// matchForLog. The real Pithing Needle exercises the omitted-ValidCards$
// default, and Alpine Moon exercises Card.Land+nonBasic.
func TestLegacyETBNameCardsReplayWithoutUniverse(t *testing.T) {
	t.Parallel()
	takeMatchSlot(t)
	reg := testutil.CorpusRegistry(t)
	needle, ok := reg.Lookup("Pithing Needle")
	if !ok {
		t.Fatal("precondition: Pithing Needle missing from corpus")
	}
	moon, ok := reg.Lookup("Alpine Moon")
	if !ok {
		t.Fatal("precondition: Alpine Moon missing from corpus")
	}
	waste, ok := reg.Lookup("Wasteland")
	if !ok {
		t.Fatal("precondition: Wasteland missing from corpus")
	}
	forest, ok := reg.Lookup("Forest")
	if !ok {
		t.Fatal("precondition: Forest missing from corpus")
	}
	deck := make([]*cards.Card, 0, 40)
	for len(deck) < 40 {
		deck = append(deck, needle, moon, waste, forest)
	}
	loader := func(name string) (Deck, error) {
		if name != "legacy-names" {
			return Deck{}, ErrNotFound
		}
		return Deck{Name: name, Cards: deck}, nil
	}
	r, err := New(Options{LoadDeck: loader, Sleep: func(time.Duration, <-chan struct{}) {}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := TableConfig{ID: "t1", Name: "legacy names", Seats: 2,
		Decks: []string{"legacy-names", "legacy-names"}, Seed: 99,
		Pace: 0, Spectator: view.Omniscient}
	if err := r.AddTable(cfg); err != nil {
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
		t.Fatalf("precondition: legacy match did not finish: %+v", m)
	}
	legacyNames := 0
	for _, ev := range m.e.L.Events {
		if ev.Counter == "name" {
			legacyNames++
		}
	}
	if legacyNames == 0 {
		t.Fatal("precondition: legacy match never resolved Pithing Needle or Alpine Moon's name choice")
	}
	sc := m.sidecar()
	if sc.NameUniverse || len(sc.NameUniverseNames) != 0 {
		t.Fatalf("legacy sidecar unexpectedly persisted a name universe: %+v", sc)
	}
	replayed, err := r.matchForLog(tb, sc, m.e.L)
	if err != nil {
		t.Fatalf("pre-feature ETB NameCard log did not replay: %v", err)
	}
	if replayed.cfg.NameUniverse != nil || replayed.cfg.NameUniverseNames != nil {
		t.Fatalf("legacy replay injected a name universe: %+v", replayed.cfg)
	}
	if got, want := replayed.e.L.Head(), m.e.L.Head(); got != want {
		t.Fatalf("legacy replay head %s, live head %s", got, want)
	}
}
