package host

import (
	"context"
	"fmt"

	"github.com/adams-shaun/gorge/engine/rules"
)

// SeriesGame is a series table's plan for one match: the deck each table
// seat plays, indexed by SEAT (never rotated by k, unlike TableConfig.
// Decks), and the seat posed CR 103.1's starting-player choice in place of
// the toss winner (rules.Config.StartChooser), or -1 to keep the seeded toss.
type SeriesGame struct {
	Decks   []string
	Chooser int
}

// SeriesFunc plans match k (k = 1 included) of a Series table. It runs on
// the table's own goroutine holding no Registry or table lock, may block —
// between the games of a series it waits for both players to sideboard —
// and must return promptly once ctx is done (Registry.Close). ok=false ends
// the table (state idle) without playing match k; an error halts it. It
// must not call Registry.Close or Registry.Wait.
type SeriesFunc func(ctx context.Context, t TableID, k int) (g SeriesGame, ok bool, err error)

// MatchTossWinner is the table seat that wins match k's CR 103.1 toss when
// no chooser is configured: rules.TossWinner over the match's engine seed
// (MatchSeed(tableSeed, k)). A SeriesFunc that reports Chooser -1 gets this
// seat for free; the value is exported so an embedder can show who would
// have won without asking the engine.
func MatchTossWinner(tableSeed uint64, k, seats int) int {
	return rules.TossWinner(MatchSeed(tableSeed, k), seats)
}

// check validates a plan against the table: exactly one deck name per seat
// (non-empty; the name itself is loaded — and a load failure halts the
// table — when newMatchWith builds the match) and -1 <= Chooser < seats.
func (g SeriesGame) check(seats int) error {
	if len(g.Decks) != seats {
		return fmt.Errorf("host: series plan has %d decks for %d seats", len(g.Decks), seats)
	}
	for i, d := range g.Decks {
		if d == "" {
			return fmt.Errorf("host: series plan seat %d has no deck", i)
		}
	}
	if g.Chooser < -1 || g.Chooser >= seats {
		return fmt.Errorf("host: series plan chooser %d, want -1..%d", g.Chooser, seats-1)
	}
	return nil
}
