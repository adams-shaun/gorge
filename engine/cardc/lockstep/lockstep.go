package lockstep

import (
	"errors"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Direction says which side plays and which replays.
type Direction uint8

const (
	Forward Direction = iota + 1 // play on Forge definitions, replay on swapped
	Reverse                      // play on swapped, replay on Forge
)

func (d Direction) String() string {
	switch d {
	case Forward:
		return "forward"
	case Reverse:
		return "reverse"
	}
	return "unknown"
}

// Swap maps a Forge-resolved deck to the deck to compare against.
type Swap func([]*cards.Card) []*cards.Card

// Game is one seated game on Forge definitions.
type Game struct {
	ID  string       // "fdn/<line>" or "legacy/<seats>"
	Cfg rules.Config // Decks hold Forge cards
}

// Result is one game in one direction.
type Result struct {
	ID      string
	Dir     Direction
	Swapped int    // distinct cards whose pointer the swap changed
	Outcome string // "clean", "diverged", "rejected" or "unfinished"
	Seq     uint64 // the first differing event, when diverged
	Detail  string // the error text, when not clean
}

// Run plays g in direction dir with newBot answering, replays the log, and reports.
func Run(g Game, dir Direction, swap Swap, newBot func(seed uint64) rules.Answerer) Result {
	forge := g.Cfg
	swapped := g.Cfg
	swapped.Decks = make([][]*cards.Card, len(g.Cfg.Decks))
	changed := map[*cards.Card]bool{}
	for i, d := range g.Cfg.Decks {
		swapped.Decks[i] = swap(d)
		for j, c := range d {
			if j < len(swapped.Decks[i]) && swapped.Decks[i][j] != c {
				changed[c] = true
			}
		}
	}
	res := Result{ID: g.ID, Dir: dir, Swapped: len(changed)}
	play, other := forge, swapped
	if dir == Reverse {
		play, other = swapped, forge
	}
	e, _, err := rules.PlayAcceptance(play, newBot, nil)
	if err != nil {
		res.Outcome, res.Detail = "unfinished", err.Error()
		return res
	}
	if _, err := replay.Replay(e.L, other); err != nil {
		var dv *replay.Divergence
		if errors.As(err, &dv) {
			res.Outcome, res.Seq, res.Detail = "diverged", dv.Seq, err.Error()
		} else {
			res.Outcome, res.Detail = "rejected", err.Error()
		}
		return res
	}
	res.Outcome = "clean"
	return res
}
