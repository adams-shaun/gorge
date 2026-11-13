package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestPlaceSeats pins the finish order a -valuelog line reports: the winner
// first, then the losers by how late they lost, a shared place for seats that
// lost on the same turn, and a shared first place for the seats still in a
// drawn game.
func TestPlaceSeats(t *testing.T) {
	won := valueLine{Winner: 2, Seats: []valueSeat{{Out: 9}, {Out: 4}, {}, {Out: 9}}}
	placeSeats(&won)
	if got := placesOf(won); got != "2 4 1 2" {
		t.Errorf("won game places %q, want \"2 4 1 2\"", got)
	}
	drawn := valueLine{Winner: -1, Seats: []valueSeat{{}, {Out: 3}, {}, {Out: 7}}}
	placeSeats(&drawn)
	if got := placesOf(drawn); got != "1 4 1 3" {
		t.Errorf("drawn game places %q, want \"1 4 1 3\"", got)
	}
}

func placesOf(l valueLine) string {
	var out []string
	for _, s := range l.Seats {
		out = append(out, string(rune('0'+s.Place)))
	}
	return strings.Join(out, " ")
}

// TestDraftsimValueLog plays one explored MBC Commander pod with -valuelog
// and -verify: the exploring drafter's picks replay from the log, and the
// pod game writes one line naming every seat's commanders, a deck without
// them, an in-hand set drawn from that deck and a winner placed first.
func TestDraftsimValueLog(t *testing.T) {
	dir := corpusReady(t)
	path := filepath.Join(t.TempDir(), "v.jsonl")
	o := options{set: "MBC", collation: "draft", format: event.FormatCommander, seed: 3, events: 1,
		players: "bot", drafter: "rating-colour", builder: "best23", verify: true, dir: dir,
		valueLog: path, pickExplore: 0.5}
	var out strings.Builder
	if _, err := run(o, &out); err != nil {
		t.Fatalf("run: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "commander+explore") {
		t.Errorf("standings do not name the exploring persona:\n%s", out.String())
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var lines []valueLine
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	for sc.Scan() {
		var l valueLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, l)
	}
	if len(lines) != 1 || len(lines[0].Seats) != 4 {
		t.Fatalf("got %d lines, want one 4-seat line", len(lines))
	}
	l := lines[0]
	if l.Winner >= 0 && l.Seats[l.Winner].Place != 1 {
		t.Errorf("winner seat %d placed %d", l.Winner, l.Seats[l.Winner].Place)
	}
	for s, vs := range l.Seats {
		if len(vs.Commanders) == 0 || len(vs.Deck) == 0 || len(vs.Hand) == 0 {
			t.Errorf("seat %d: %d commanders, %d deck cards, %d in hand", s, len(vs.Commanders), len(vs.Deck), len(vs.Hand))
		}
		inDeck := map[string]bool{}
		for _, c := range vs.Deck {
			inDeck[c] = true
		}
		for _, c := range vs.Commanders {
			if inDeck[c] {
				t.Errorf("seat %d: commander %s listed in its deck", s, c)
			}
			inDeck[c] = true
		}
		for _, c := range vs.Hand {
			if !inDeck[c] {
				t.Errorf("seat %d: in-hand %s is not one of its cards", s, c)
			}
		}
	}
}
