package main

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestReviewLoadsABenchRecord is phase 1 end to end: a bench game recorded
// through Hooks.Record (T1) loads through reviewLoad into a review table
// (T2), whose omniscient views carry every hand and no library.
func TestReviewLoadsABenchRecord(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{"mono-black-aggro", "mono-red-prowess"}
	decks := make([][]*cards.Card, len(names))
	for i, n := range names {
		d, err := testutil.LoadRepoDeck(reg, n)
		if err != nil {
			t.Fatal(err)
		}
		decks[i] = d
	}
	cfg := rules.Config{Seed: 424242, Names: names, Decks: decks, Tokens: reg.Tokens}
	path := filepath.Join(t.TempDir(), "g0"+matchrecord.Ext)
	sink := &matchrecord.Sink{
		Producer: matchrecord.Producer{Tool: "review-test", Policies: []string{"heuristic", "heuristic"}},
		Put:      func(r *matchrecord.Record) error { return matchrecord.Write(path, r) },
	}
	seats := []seat.Seat{heuristic.NewBot(cfg.Seed ^ 1), heuristic.NewBot(cfg.Seed ^ 2)}
	_, e, err := bench.PlayGame(cfg, seats, 60, 6000, bench.Hooks{Record: sink})
	if err != nil {
		t.Fatal(err)
	}
	m, err := reviewLoad(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Policies) != 2 || len(m.Names) != 2 {
		t.Fatalf("policies %v names %v", m.Policies, m.Names)
	}
	r, err := host.New(host.Options{LoadDeck: func(string) (host.Deck, error) { return host.Deck{}, nil }})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddReviewTable("g0", m); err != nil {
		t.Fatal(err)
	}
	last := uint64(len(e.L.Events) - 1)
	for _, seq := range []uint64{10, last / 2, last} {
		v, err := r.ViewAt("g0", 1, seq)
		if err != nil {
			t.Fatalf("ViewAt(%d): %v", seq, err)
		}
		for _, p := range v.Players {
			if p.Hand == nil || len(p.Hand) != p.HandSize || p.Library != nil || p.LibraryTop != nil {
				t.Fatalf("seq %d seat %d: hand %v (size %d), library %v -- want every hand, no library", seq, p.ID, p.Hand, p.HandSize, p.Library)
			}
		}
	}
}
