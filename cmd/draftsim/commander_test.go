package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDraftsimCommanderPod is the Commander-draft end-to-end check: an MBC
// pod of four drafts its 20-card packs two picks per pass, builds 60-card
// decks around drafted commanders, plays one 4-player free-for-all
// Commander game at 40 life with -verify, and logs the game as a
// PodGameEnded whose head replays.
func TestDraftsimCommanderPod(t *testing.T) {
	dir := corpusReady(t)
	o := options{set: "MBC", collation: "draft", format: event.FormatCommander, seed: 1, events: 1,
		players: "bot", drafter: "rating-colour", builder: "best23", verify: true, dir: dir}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	t.Log(out.String())
	ev, err := event.ReplayLog(res.events[0].log)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Format() != event.FormatCommander || ev.PicksPerPass() != 2 || ev.Structure() != event.StructurePod {
		t.Fatalf("event shape = %q, %d picks, %q; want commander, 2, pod", ev.Format(), ev.PicksPerPass(), ev.Structure())
	}
	for s := 0; s < 4; s++ {
		if n := len(ev.Pool(s)); n != 60 {
			t.Fatalf("seat %d pool = %d cards, want 60 (3 packs of 20)", s, n)
		}
		d, _ := ev.SavedDeck(s)
		if err := build.ValidateCommander(ev.Pool(s), d); err != nil {
			t.Fatalf("seat %d: %v", s, err)
		}
	}
	games := 0
	for _, en := range ev.Log() {
		if en.Kind == event.KindPodGameEnded {
			games++
		}
	}
	if games != 1 || len(res.events[0].games) != 1 || !res.events[0].games[0].verified {
		t.Fatalf("pod games logged %d, played %d; want one verified game", games, len(res.events[0].games))
	}
	if !strings.Contains(out.String(), "4-player commander") {
		t.Fatalf("no pod game line in the output:\n%s", out.String())
	}
}

// TestDraftsimCommanderShapeRefusals pins the -format commander flag checks,
// all refused before the corpus opens.
func TestDraftsimCommanderShapeRefusals(t *testing.T) {
	for _, o := range []options{
		{format: "brawl"},
		{format: event.FormatCommander, seats: 8},
		{format: event.FormatCommander, mode: event.ModeSealed},
		{format: event.FormatCommander, structure: event.StructureSwiss},
		{format: event.FormatCommander, rounds: event.MaxPodGames + 1},
		{format: event.FormatCommander, picks: 9},
		{deckSize: 50},
		{format: event.FormatCommander, deckSize: build.MinCommanderDeckSize - 1},
		{format: event.FormatCommander, deckSize: build.MaxDeckSize + 1},
		{format: event.FormatCommander, structure: event.StructureNone, rounds: 1},
	} {
		if _, err := o.shape(); err == nil {
			t.Errorf("shape(%+v) accepted, want a refusal", o)
		}
	}
	sh, err := options{format: event.FormatCommander}.shape()
	if err != nil || sh.picks != 2 || sh.rounds != 1 || sh.structure != event.StructurePod || sh.seats != 4 {
		t.Fatalf("default commander shape = %+v, %v; want 4 seats, 2 picks, 1 pod game", sh, err)
	}
	sh, err = options{format: event.FormatCommander, structure: event.StructureNone, deckSize: 50}.shape()
	if err != nil || sh.structure != event.StructureNone || sh.rounds != 0 || sh.deckSize != 50 {
		t.Fatalf("commander build-only shape = %+v, %v; want structure none, no rounds, deck size 50", sh, err)
	}
	if sh, err := (options{format: event.FormatCommander, deckSize: build.CommanderDeckSize}).shape(); err != nil || sh.deckSize != 0 {
		t.Fatalf("-deck-size 60 shape = %+v, %v; want the default (0), so the log omits it", sh, err)
	}
}

// TestDraftsimCommanderDecksOut is the deck-pool path: an MBC Commander
// draft at -deck-size 50 with -structure none builds without playing, logs
// deck_size 50, and -decks-out writes one deck JSON file per seat, named
// <set>_e<seed>_s<seat>.json, of exactly 50 cards whose commanders resolve,
// are a legal configuration and hold every card's colour identity. A second
// run writes the same bytes.
func TestDraftsimCommanderDecksOut(t *testing.T) {
	dir := corpusReady(t)
	run1 := func(out string) *result {
		t.Helper()
		o := options{set: "MBC", collation: "draft", format: event.FormatCommander, deckSize: 50,
			structure: event.StructureNone, seed: 1, events: 1, players: "bot",
			drafter: "rating-colour", builder: "best23", verify: true, decksOut: out, dir: dir}
		var log strings.Builder
		res, err := run(o, &log)
		if err != nil {
			t.Fatalf("run: %v\n%s", err, log.String())
		}
		t.Log(log.String())
		return res
	}
	outA, outB := t.TempDir(), t.TempDir()
	res := run1(outA)
	run1(outB)
	ev, err := event.ReplayLog(res.events[0].log)
	if err != nil {
		t.Fatal(err)
	}
	if ev.DeckSize() != 50 || ev.Structure() != event.StructureNone || len(res.events[0].games) != 0 {
		t.Fatalf("event deck size %d, structure %q, %d games; want 50, none, 0", ev.DeckSize(), ev.Structure(), len(res.events[0].games))
	}
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatal(err)
	}
	names, _ := filepath.Glob(filepath.Join(outA, "*.json"))
	if len(names) != 4 {
		t.Fatalf("-decks-out wrote %v, want 4 files", names)
	}
	for s := 0; s < 4; s++ {
		name := deckFileName("MBC", 1, s)
		if name != "mbc_e1_s"+string(rune('0'+s))+".json" {
			t.Fatalf("deck file name %q", name)
		}
		raw, err := os.ReadFile(filepath.Join(outA, name))
		if err != nil {
			t.Fatal(err)
		}
		again, err := os.ReadFile(filepath.Join(outB, name))
		if err != nil || !bytes.Equal(raw, again) {
			t.Fatalf("seat %d: a second run wrote different bytes (%v)", s, err)
		}
		f, err := deck.Parse(raw)
		if err != nil {
			t.Fatalf("seat %d: %v", s, err)
		}
		cs, err := f.Resolve(reg)
		if err != nil {
			t.Fatalf("seat %d resolve: %v", s, err)
		}
		if len(cs) != 50 || f.Format != "commander" {
			t.Fatalf("seat %d deck file = %d cards, format %q; want 50, commander", s, len(cs), f.Format)
		}
		if err := checkCommanderDeck(cs, f.CommanderIndices()); err != nil {
			t.Fatalf("seat %d: %v", s, err)
		}
		d, _ := ev.SavedDeck(s)
		if err := build.ValidateCommanderSize(ev.Pool(s), d, 50); err != nil {
			t.Fatalf("seat %d: %v", s, err)
		}
		if len(f.CommanderNames()) != len(d.Commanders) {
			t.Fatalf("seat %d file commanders %v, deck %v", s, f.CommanderNames(), d.Commanders)
		}
	}
}
