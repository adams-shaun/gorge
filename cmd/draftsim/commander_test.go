package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/build"
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
	} {
		if _, err := o.shape(); err == nil {
			t.Errorf("shape(%+v) accepted, want a refusal", o)
		}
	}
	sh, err := options{format: event.FormatCommander}.shape()
	if err != nil || sh.picks != 2 || sh.rounds != 1 || sh.structure != event.StructurePod || sh.seats != 4 {
		t.Fatalf("default commander shape = %+v, %v; want 4 seats, 2 picks, 1 pod game", sh, err)
	}
}
