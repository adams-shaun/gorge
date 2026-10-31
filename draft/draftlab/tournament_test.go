package draftlab

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// TestPlayRoundRobinFour: 4 entrants play a 3-round round robin, 6 games in
// all, with the event's own pairings and 4 standings rows.
func TestPlayRoundRobinFour(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 4)
	tn, err := l.Play(es, PlayOptions{Structure: RoundRobin}, 555)
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if tn.Structure != RoundRobin {
		t.Errorf("Structure = %q, want %q", tn.Structure, RoundRobin)
	}
	if tn.Rounds != 3 {
		t.Errorf("Rounds = %d, want 3", tn.Rounds)
	}
	if len(tn.Games) != 6 {
		t.Errorf("Games = %d, want 6", len(tn.Games))
	}
	if len(tn.Standings) != 4 {
		t.Errorf("Standings rows = %d, want 4", len(tn.Standings))
	}
	if len(tn.Pairings) != 3 {
		t.Fatalf("Pairings rounds = %d, want 3", len(tn.Pairings))
	}
	for r := 1; r <= 3; r++ {
		want := event.RoundRobinPairings(4, r)
		if !reflect.DeepEqual(tn.Pairings[r-1], want) {
			t.Errorf("round %d pairings = %v, want %v", r, tn.Pairings[r-1], want)
		}
	}
	for _, g := range tn.Games {
		if g.Round < 1 || g.Round > 3 || g.Game != 1 {
			t.Errorf("game Round/Game = %d/%d, want 1..3/1", g.Round, g.Game)
		}
		if g.A < 0 || g.A >= 4 || g.B < 0 || g.B >= 4 || g.A >= g.B {
			t.Errorf("game A/B = %d/%d, want a valid lower-first entrant pair", g.A, g.B)
		}
	}
}

// TestPlaySwissEight: 8 sealed-built entrants play a 3-round Swiss, 12 games,
// no pairing repeats, 8 standings rows; a second run is identical.
func TestPlaySwissEight(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 8)
	tn, err := l.Play(es, PlayOptions{Structure: Swiss}, 808)
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if tn.Rounds != 3 {
		t.Errorf("Rounds = %d, want 3", tn.Rounds)
	}
	if len(tn.Games) != 12 {
		t.Errorf("Games = %d, want 12", len(tn.Games))
	}
	if len(tn.Standings) != 8 {
		t.Errorf("Standings rows = %d, want 8", len(tn.Standings))
	}
	seen := make(map[[2]int]bool)
	for r, round := range tn.Pairings {
		if len(round) != 4 {
			t.Errorf("round %d has %d pairs, want 4", r+1, len(round))
		}
		for _, p := range round {
			if seen[p] {
				t.Errorf("pair %v repeats", p)
			}
			seen[p] = true
		}
	}
	if len(seen) != 12 {
		t.Errorf("distinct pairs = %d, want 12", len(seen))
	}

	tn2, err := l.Play(es, PlayOptions{Structure: Swiss}, 808)
	if err != nil {
		t.Fatalf("Play (second run): %v", err)
	}
	if !reflect.DeepEqual(tn, tn2) {
		t.Errorf("two Swiss runs at the same seed differ")
	}
}

// TestPlayNoGames: the none structure plays no games and returns n zero
// standings rows.
func TestPlayNoGames(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 4)
	tn, err := l.Play(es, PlayOptions{Structure: NoGames}, 1)
	if err != nil {
		t.Fatalf("Play: %v", err)
	}
	if tn.Rounds != 0 || len(tn.Games) != 0 || len(tn.Pairings) != 0 {
		t.Errorf("NoGames: rounds=%d games=%d pairing rounds=%d, want 0/0/0", tn.Rounds, len(tn.Games), len(tn.Pairings))
	}
	if len(tn.Standings) != 4 {
		t.Fatalf("Standings rows = %d, want 4", len(tn.Standings))
	}
	for _, s := range tn.Standings {
		if s.Points != 0 {
			t.Errorf("seat %d has %d points, want 0", s.Seat, s.Points)
		}
	}
	// A none structure rejects an explicit round count.
	if _, err := l.Play(es, PlayOptions{Structure: NoGames, Rounds: 2}, 1); err == nil {
		t.Error("NoGames accepted an explicit round count")
	}
}

// TestPlayDecksShape: PlayDecks resolves one player for every deck or one
// per deck; a bad player count is refused.
func TestPlayDecksShape(t *testing.T) {
	l := testLab(t)
	decks, err := l.CatalogDecks("fdn-limited")
	if err != nil {
		t.Fatalf("CatalogDecks: %v", err)
	}
	if len(decks) < 2 {
		t.Fatalf("catalog has %d decks, want >= 2 (precondition)", len(decks))
	}
	decks = decks[:2]

	one, err := l.PlayDecks(decks, []AgentRef{{Name: "bot"}}, RoundRobin, 321)
	if err != nil {
		t.Fatalf("PlayDecks one player: %v", err)
	}
	if one.Rounds != 1 || len(one.Games) != 1 || len(one.Standings) != 2 {
		t.Errorf("PlayDecks shape: rounds=%d games=%d standings=%d, want 1/1/2", one.Rounds, len(one.Games), len(one.Standings))
	}

	per, err := l.PlayDecks(decks, []AgentRef{{Name: "bot"}, {Name: "bot"}}, RoundRobin, 321)
	if err != nil {
		t.Fatalf("PlayDecks per-deck players: %v", err)
	}
	if !reflect.DeepEqual(one, per) {
		t.Errorf("PlayDecks differs between one shared player and one per deck at the same seed")
	}

	if _, err := l.PlayDecks(decks, nil, RoundRobin, 1); err == nil {
		t.Error("PlayDecks accepted no players")
	}
	if _, err := l.PlayDecks(decks, []AgentRef{{Name: "bot"}, {Name: "bot"}, {Name: "bot"}}, RoundRobin, 1); err == nil {
		t.Error("PlayDecks accepted 3 players for 2 decks")
	}
}
