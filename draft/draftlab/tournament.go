package draftlab

import (
	"fmt"

	"github.com/adams-shaun/gorge/draft/event"
)

// Structure is a tournament's pairings schedule.
type Structure string

// The three structures Play knows.
const (
	// RoundRobin pairs every entrant with every other once (n-1 rounds).
	RoundRobin Structure = "roundrobin"
	// Swiss pairs by standing with a seeded tie-break (ceil(log2 n) rounds
	// by default), never repeating a pairing while a rematch-free matching
	// exists.
	Swiss Structure = "swiss"
	// NoGames runs the structure's shape but plays no games (zero rounds).
	NoGames Structure = "none"
)

// maxEntrants bounds a tournament's field: Play wants an even count in
// 2..maxEntrants.
const maxEntrants = 64

// PlayOptions shapes a tournament. Rounds 0 means the structure's default
// (round robin: n-1; Swiss: ceil(log2 n); none: 0).
type PlayOptions struct {
	Structure Structure
	Rounds    int
}

// Tournament is one finished Play: the structure and round count, the pairs
// of each round, the played games in round then pairing order, and the final
// standings table.
type Tournament struct {
	Structure Structure
	Rounds    int
	Pairings  [][][2]int   // per round
	Games     []GameResult // round order, pairing order within a round
	Standings []event.Standing
	// Manifest is the run's provenance (kind "tournament").
	Manifest Manifest
}

// Play runs a Bo1 tournament between an even number (2..maxEntrants) of
// entrants. Round r's pairs come from event.RoundRobinPairings or
// event.SwissPairings; every pair (a, b) is one Bo1 game with entrant a as
// player 0 and engine seed gameSeed(seed, r, a, b, 1). A round's games run
// in parallel (Options.Workers) and are recorded in pairing order, so no
// result depends on the worker count. A stall counts as a drawn game, as
// cmd/draftsim logs it.
func (l *Lab) Play(entrants []Entrant, o PlayOptions, seed uint64) (Tournament, error) {
	n := len(entrants)
	if n < 2 || n > maxEntrants || n%2 != 0 {
		return Tournament{}, fmt.Errorf("draftlab: play: %d entrants, want an even count in 2..%d", n, maxEntrants)
	}
	structure := o.Structure
	if structure == "" {
		structure = RoundRobin
	}
	rounds, err := playRounds(structure, n, o.Rounds)
	if err != nil {
		return Tournament{}, err
	}
	t := Tournament{Structure: structure, Rounds: rounds}
	if structure == NoGames {
		t.Standings = event.ComputeStandingsN(n, nil)
		t.Manifest = l.NewManifest("tournament", seed, playerRoleRefs(entrants), nil)
		return t, nil
	}
	var results []event.SeriesResult
	for r := 1; r <= rounds; r++ {
		var pairs [][2]int
		switch structure {
		case RoundRobin:
			pairs = event.RoundRobinPairings(n, r)
		case Swiss:
			pairs, err = event.SwissPairings(n, r, results, seed)
			if err != nil {
				return Tournament{}, fmt.Errorf("draftlab: play: swiss round %d: %w", r, err)
			}
		}
		roundGames, err := l.runGames(len(pairs), func(i int) (GameResult, error) {
			a, b := pairs[i][0], pairs[i][1]
			gs := gameSeed(seed, r, a, b, 1)
			res, err := l.playGame(entrants[a], entrants[b], [2]string{fmt.Sprintf("p%d", a), fmt.Sprintf("p%d", b)}, gs, false)
			if err != nil {
				return GameResult{}, err
			}
			res.Round, res.Game, res.A, res.B = r, 1, a, b
			return res, nil
		})
		if err != nil {
			return Tournament{}, fmt.Errorf("draftlab: play: round %d: %w", r, err)
		}
		t.Pairings = append(t.Pairings, pairs)
		for i, pair := range pairs {
			g := roundGames[i]
			sr := event.SeriesResult{Round: r, A: pair[0], B: pair[1]}
			switch g.Winner {
			case 0:
				sr.Score[0] = 1
				sr.Games = []event.GameOutcome{event.AWon}
			case 1:
				sr.Score[1] = 1
				sr.Games = []event.GameOutcome{event.BWon}
			default:
				sr.Games = []event.GameOutcome{event.Draw}
			}
			results = append(results, sr)
			t.Games = append(t.Games, g)
		}
	}
	t.Standings = event.ComputeStandingsN(n, results)
	t.Manifest = l.NewManifest("tournament", seed, playerRoleRefs(entrants), t.Games)
	return t, nil
}

// PlayDecks is Play over decks and player refs: players has one entry (every
// deck plays it) or len(decks). Each deck is paired with its resolved player
// via Lab.Player.
func (l *Lab) PlayDecks(decks []Deck, players []AgentRef, s Structure, seed uint64) (Tournament, error) {
	if len(decks) == 0 {
		return Tournament{}, fmt.Errorf("draftlab: PlayDecks: no decks")
	}
	if len(players) != 1 && len(players) != len(decks) {
		return Tournament{}, fmt.Errorf("draftlab: PlayDecks: %d players for %d decks, want 1 or %d", len(players), len(decks), len(decks))
	}
	entrants := make([]Entrant, len(decks))
	for i := range decks {
		ref := players[0]
		if len(players) == len(decks) {
			ref = players[i]
		}
		np, err := l.Player(ref)
		if err != nil {
			return Tournament{}, fmt.Errorf("draftlab: PlayDecks: deck %d: %w", i, err)
		}
		entrants[i] = Entrant{Deck: decks[i], Player: np}
	}
	return l.Play(entrants, PlayOptions{Structure: s}, seed)
}

// playRounds resolves a structure's round count: the explicit value when set
// (validated 1..n-1, as event.EventCreated validates it), else the default
// (n-1 for round robin, ceil(log2 n) for Swiss, 0 for none).
func playRounds(structure Structure, n, rounds int) (int, error) {
	switch structure {
	case NoGames:
		if rounds != 0 {
			return 0, fmt.Errorf("draftlab: play: structure none takes no rounds, got %d", rounds)
		}
		return 0, nil
	case RoundRobin, Swiss:
		if rounds == 0 {
			if structure == Swiss {
				return swissRounds(n), nil
			}
			return n - 1, nil
		}
		if rounds < 1 || rounds >= n {
			return 0, fmt.Errorf("draftlab: play: rounds %d out of range 1..%d", rounds, n-1)
		}
		return rounds, nil
	default:
		return 0, fmt.Errorf("draftlab: play: unknown structure %q", structure)
	}
}

// swissRounds is a Swiss event's default round count: ceil(log2 n).
func swissRounds(n int) int {
	for r := 0; ; r++ {
		if 1<<uint(r) >= n {
			return r
		}
	}
}
