package main

// The feedback sideboard round-trip (task sideboards, fix round 2):
// sideboards are genesis configuration — the engine mints their objects
// before the first event — so a captured match.json must record the
// sideboard card names beside deck_cards, and the replay rebuild
// (feedback.Load → rules.Config) must restore them, or the genesis object
// IDs shift and every captured Burning Wish/Karn match diverges at the
// first shuffle. This test captures a live snapshot from a table whose
// deck file carries a sideboard (the-epic-storm, the repo deck that plays
// Burning Wish) and replays it through the same Load the CLI uses.

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/internal/testutil/feedback"
	"github.com/adams-shaun/gorge/replay"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

const sideboardReplayDeck = "the-epic-storm"

func TestReproSideboardCaptureReplaysWithSideboards(t *testing.T) {
	requireCorpus(t)
	root, err := feedback.Root()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	reg := testutil.CorpusRegistry(t)
	f := testutil.RepoDeckFile(t, sideboardReplayDeck)
	cs, err := f.Resolve(reg)
	if err != nil {
		t.Fatalf("deck resolve: %v", err)
	}
	sb, err := f.ResolveSideboard(reg)
	if err != nil {
		t.Fatalf("sideboard resolve: %v", err)
	}
	if len(sb) == 0 {
		t.Fatalf("%s carries no sideboard — the fixture deck must have one", sideboardReplayDeck)
	}
	wantSB := make([]string, len(sb))
	for i, c := range sb {
		wantSB[i] = c.Faces[0].Name
	}
	by := map[string]host.Deck{
		sideboardReplayDeck: {Name: sideboardReplayDeck, Cards: cs, Sideboard: sb},
	}
	load := func(name string) (host.Deck, error) {
		d, ok := by[name]
		if !ok {
			return host.Deck{}, host.ErrNotFound
		}
		return d, nil
	}
	gate := &gateSeat{bot: seat.NewBot(1), reached: make(chan struct{}), release: make(chan struct{})}
	r, err := host.New(host.Options{
		LoadDeck: load,
		Tokens:   reg.Tokens,
		Seats: func(seatNames []string, seed uint64) []seat.Seat {
			return []seat.Seat{gate, seat.NewBot(seed ^ 2)}
		},
		Sleep: func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	if err := r.AddTable(host.TableConfig{ID: "t1", Name: "Table t1", Seats: 2,
		Decks: []string{sideboardReplayDeck, sideboardReplayDeck}, Seed: 42,
		Spectator: view.Public}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	// Record a couple of real intents (bot decisions behind the gate) so the
	// log carries post-genesis card references the replay must reproduce.
	for i := 0; i < 2; i++ {
		select {
		case <-gate.reached:
		case <-time.After(30 * time.Second):
			t.Fatalf("seat 0 never reached decision %d", i+1)
		}
		gate.release <- struct{}{}
	}
	select {
	case <-gate.reached:
	case <-time.After(30 * time.Second):
		t.Fatal("seat 0 never reached its post-advance decision; capture point not quiescent")
	}

	dir := t.TempDir()
	captureSnapshotFiles(t, r, dir)

	// The recorded match.json must name the sideboard contents.
	l, cfg, _, err := feedback.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Sideboards) != 2 {
		t.Fatalf("rebuilt config carries %d sideboards, want one per seat", len(cfg.Sideboards))
	}
	for seat := 0; seat < 2; seat++ {
		got := make([]string, 0, len(cfg.Sideboards[seat]))
		for _, c := range cfg.Sideboards[seat] {
			got = append(got, c.Faces[0].Name)
		}
		if len(got) != len(wantSB) {
			t.Fatalf("seat %d rebuilt sideboard = %v, want %v", seat, got, wantSB)
		}
		for i := range wantSB {
			if got[i] != wantSB[i] {
				t.Fatalf("seat %d rebuilt sideboard = %v, want %v", seat, got, wantSB)
			}
		}
	}
	// The full replay must reproduce the recorded event stream.
	full, err := replay.Replay(l, cfg)
	if err != nil {
		t.Fatalf("full replay: %v", err)
	}
	// And the replayed genesis must hold the sideboard cards where the
	// capture's engine did — the zone a wish searches.
	for seat := 0; seat < 2; seat++ {
		sbZone := full.G.Zone(state.ZSideboard, state.PlayerID(seat))
		if len(sbZone) != len(wantSB) {
			t.Fatalf("replayed seat %d sideboard holds %d cards, want %d", seat, len(sbZone), len(wantSB))
		}
		for i, id := range sbZone {
			o := full.G.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != wantSB[i] {
				t.Fatalf("replayed seat %d sideboard slot %d = %+v, want %q", seat, i, o, wantSB[i])
			}
		}
	}
}
