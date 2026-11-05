package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
)

// TestDraftsimPrereleaseShape: -mode prerelease resolves to sealed mode,
// six packs and one promo, and -draftlog with it is refused.
func TestDraftsimPrereleaseShape(t *testing.T) {
	sh, err := options{mode: "prerelease", seats: 4}.shape()
	if err != nil {
		t.Fatalf("shape: %v", err)
	}
	if sh.mode != event.ModeSealed || sh.packs != 6 || sh.promos != 1 {
		t.Fatalf("shape = mode %q packs %d promos %d, want sealed 6 1", sh.mode, sh.packs, sh.promos)
	}
	sealed, err := options{mode: "sealed", seats: 4}.shape()
	if err != nil || sealed.promos != 0 {
		t.Fatalf("sealed shape = %+v, %v; want no promos", sealed, err)
	}
	_, err = options{mode: "prerelease", seats: 4, draftlog: "x.csv"}.shape()
	if err == nil || !strings.Contains(err.Error(), "-draftlog needs -mode draft") {
		t.Fatalf("prerelease -draftlog err = %v, want the -draftlog refusal", err)
	}
	_, err = options{mode: "bogus", seats: 4}.shape()
	if err == nil || !strings.Contains(err.Error(), "draft, sealed or prerelease") {
		t.Fatalf("unknown mode err = %v, want it to list prerelease", err)
	}
}

// TestDraftsimPrereleaseBuildsOnly: -mode prerelease -structure none over
// four seats prints the prerelease header, opens six boosters and a promo
// per seat, and every event verifies.
func TestDraftsimPrereleaseBuildsOnly(t *testing.T) {
	dir := corpusReady(t)
	o := options{
		set: "FDN", seed: 1, events: 1, seats: 4, mode: "prerelease", structure: "none",
		players: "bot", drafter: "rating-colour", builder: "best23", verify: true, dir: dir,
	}
	var out strings.Builder
	res, err := run(o, &out)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	header, _, _ := strings.Cut(out.String(), "\n")
	if !strings.Contains(header, ", mode prerelease") || strings.Contains(header, "mode sealed") {
		t.Fatalf("header %q, want ', mode prerelease' in place of sealed", header)
	}
	if len(res.events) != 1 {
		t.Fatalf("%d events, want 1", len(res.events))
	}
	er := res.events[0]
	entries, err := event.ParseLog(er.log)
	if err != nil {
		t.Fatalf("ParseLog: %v", err)
	}
	if entries[0].Promos != 1 {
		t.Fatalf("EventCreated promos = %d, want 1", entries[0].Promos)
	}
	opened := 0
	for _, en := range entries {
		if en.Kind != event.KindPackOpened {
			continue
		}
		opened++
		if en.Round == 7 {
			if len(en.Cards) != 1 || (en.Cards[0].Rarity != limited.Rare && en.Cards[0].Rarity != limited.Mythic) {
				t.Fatalf("seat %d promo pack %+v, want one R/M card", en.Seat, en.Cards)
			}
		}
	}
	if opened != 4*7 {
		t.Fatalf("%d PackOpened, want %d", opened, 4*7)
	}
	if _, err := event.ReplayLog(er.log); err != nil {
		t.Fatalf("ReplayLog: %v", err)
	}
	if len(er.decks) != 4 || len(er.games) != 0 {
		t.Fatalf("%d decks %d games, want 4 and 0", len(er.decks), len(er.games))
	}
}
