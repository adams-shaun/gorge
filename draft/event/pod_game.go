package event

import (
	"fmt"
	"sort"

	"github.com/adams-shaun/gorge/draft/limited/build"
)

// The Commander-draft event shape (Commander Legends / Mystery Booster
// Commander drafts): FormatCommander decks are built around one or two
// drafted commanders (build.ValidateCommanderSize, at least DeckSize
// cards), and StructurePod plays the whole pod at one multiplayer
// free-for-all table per event round.
const (
	// FormatCommander is the EventCreated Format of a Commander draft.
	FormatCommander = "commander"
	// StructurePod plays Rounds free-for-all games, every seat at one
	// table, each reported as a PodGameEnded entry.
	StructurePod = "pod"
	// MaxPodGames bounds a StructurePod event's round count.
	MaxPodGames = 16
)

// podResult is one played free-for-all game.
type podResult struct {
	round  int
	winner int // -1 for a draw (including a watchdog stall)
	head   string
}

// Format reports the event's construction format: FormatCommander, or ""
// for a limited (40-card) event.
func (e *Event) Format() string { return e.format }

// DeckSize reports a Commander-draft event's minimum deck size: the
// EventCreated DeckSize, or build.CommanderDeckSize when it named none. A
// limited event reports 0 (its decks follow build.Validate).
func (e *Event) DeckSize() int {
	if e.format != FormatCommander {
		return 0
	}
	if e.deckSize == 0 {
		return build.CommanderDeckSize
	}
	return e.deckSize
}

// PicksPerPass reports how many cards a seat takes from a pack before
// passing it: 1 unless the EventCreated entry named more.
func (e *Event) PicksPerPass() int {
	if e.picksPerPass == 0 {
		return 1
	}
	return e.picksPerPass
}

// PendingPodGame reports the event round whose free-for-all game the driver
// must play next: (r, true) in a StructurePod event's round r until its
// PodGameEnded is logged.
func (e *Event) PendingPodGame() (round int, ok bool) {
	if e.structure != StructurePod || !e.inRound() {
		return 0, false
	}
	if len(e.podResults) >= e.round {
		return 0, false
	}
	return e.round, true
}

// validateDeck checks a DeckSaved deck against the seat's pool by the
// event's format.
func (e *Event) validateDeck(seat int, d build.Deck) error {
	if e.format == FormatCommander {
		return build.ValidateCommanderSize(e.Pool(seat), d, e.DeckSize())
	}
	return build.Validate(e.Pool(seat), d)
}

// applyPodGameEnded records one free-for-all game of a StructurePod event
// and opens the next round, or leaves the last round awaiting EventEnded.
func (e *Event) applyPodGameEnded(en *Entry) error {
	if !e.cooked {
		return fmt.Errorf("event: PodGameEnded: event not created")
	}
	if e.structure != StructurePod {
		return fmt.Errorf("event: PodGameEnded: structure %s plays no pod games", e.structure)
	}
	round, ok := e.PendingPodGame()
	if !ok {
		return fmt.Errorf("event: PodGameEnded: no pod game pending in phase %s", e.phase)
	}
	if en.Round != round {
		return fmt.Errorf("event: PodGameEnded: round %d is not the current round %d", en.Round, round)
	}
	w := -1
	if en.Winner != nil {
		w = *en.Winner
		if w < 0 || w >= e.NumSeats() {
			return fmt.Errorf("event: PodGameEnded: winner seat %d out of range", w)
		}
	}
	e.podResults = append(e.podResults, podResult{round: round, winner: w, head: en.Head})
	if e.round < e.rounds {
		e.round++
		e.phase = RoundPhase(e.round)
	}
	return nil
}

// podStandings is a StructurePod event's table: 3 points per win and 1 per
// drawn game for every seat at it; MW and GW are points over 3×games (no
// floor: a free-for-all has no match-win percentage to protect), and OMW
// and OGW stay 0. Ordered by points, then seat.
func (e *Event) podStandings() []Standing {
	n := e.NumSeats()
	points := make([]int, n)
	for _, r := range e.podResults {
		if r.winner < 0 {
			for s := range points {
				points[s]++
			}
			continue
		}
		points[r.winner] += 3
	}
	games := len(e.podResults)
	out := make([]Standing, n)
	for s := range out {
		out[s] = Standing{Seat: s, Points: points[s]}
		if games > 0 {
			out[s].MW = float64(points[s]) / (3 * float64(games))
			out[s].GW = out[s].MW
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Points != out[j].Points {
			return out[i].Points > out[j].Points
		}
		return out[i].Seat < out[j].Seat
	})
	return out
}
