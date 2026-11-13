package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"sort"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/bench"
)

// The -valuelog and -pick-explore flags feed card valuation from simulated
// Commander-draft pods: every pod game writes one JSON line naming each
// seat's commanders, its deck, the cards it held in hand and where it
// finished, and the win/loss regression over those lines is offline.

// exploreDrafter wraps a drafter so that, with probability p per pick, the
// seat takes a uniformly random pack card instead. The persona alone
// drafts the same cards for the same packs, so a card it never rates highly
// would never reach a deck and could not be valued; exploration widens the
// decks the games sample. The draw comes from the seat's own seeded rand,
// so the event stays reproducible from its seed, and the picks are logged
// as usual, so -verify replays them.
type exploreDrafter struct {
	inner pickbot.Drafter
	p     float64
}

func (d *exploreDrafter) Pick(v pickbot.PickView, r *rand.Rand) string {
	if len(v.Pack) > 0 && r.Float64() < d.p {
		return v.Pack[r.IntN(len(v.Pack))].ID
	}
	return d.inner.Pick(v, r)
}

// valueSeat is one seat's half of a -valuelog line.
type valueSeat struct {
	// Commanders and Deck name the seat's cards; Deck leaves out the
	// commanders and basic lands and repeats a name per copy.
	Commanders []string `json:"c"`
	Deck       []string `json:"d"`
	// Hand is the seat's in-hand set, the 17Lands GIH set: the opening hand
	// after mulligans plus every card later moved from its library to its
	// hand, basic lands left out, one name per card object, sorted.
	Hand []string `json:"h"`
	// Cast counts the times the seat cast one of its commanders.
	Cast int `json:"cc"`
	// Out is the turn count at which the seat lost, 0 for a seat still in
	// the game at its end; Place is its finish, 1 for the winner, ties for
	// seats still in a drawn or stalled game.
	Out   int `json:"out"`
	Place int `json:"pl"`
}

// valueLine is one pod game's -valuelog line.
type valueLine struct {
	Event   uint64      `json:"ev"`
	Round   int         `json:"g"`
	Seed    uint64      `json:"seed"`
	Turns   int         `json:"turns"`
	Stall   string      `json:"stall,omitempty"`
	Winner  int         `json:"w"` // -1 for a draw or a stall
	Starter int         `json:"st"`
	Seats   []valueSeat `json:"seats"`
}

// writeValueLine writes one pod game's -valuelog line.
func writeValueLine(w io.Writer, evSeed uint64, round int, seed uint64, oc bench.Outcome, eng *rules.Engine, decks []seatDeck) error {
	line := valueLine{Event: evSeed, Round: round, Seed: seed, Turns: int(oc.Turns), Stall: oc.StallOn, Winner: -1, Starter: -1}
	if oc.StarterSet {
		line.Starter = oc.Starter
	}
	if oc.StallOn == "" && !oc.Draw {
		line.Winner = oc.WinnerSeat
	}
	line.Seats = make([]valueSeat, len(decks))
	for s, d := range decks {
		vs := &line.Seats[s]
		isCmdr := map[int]bool{}
		for _, i := range d.commanders {
			isCmdr[i] = true
			vs.Commanders = append(vs.Commanders, d.cards[i].Faces[0].Name)
		}
		for i, c := range d.cards {
			if isCmdr[i] || len(c.Faces) == 0 || (c.Faces[0].IsBasic() && c.Faces[0].IsLand()) {
				continue
			}
			vs.Deck = append(vs.Deck, c.Faces[0].Name)
		}
		vs.Hand = []string{}
	}
	hands, cast, outTurn := gameFacts(eng, len(decks))
	for s := range line.Seats {
		line.Seats[s].Hand = append(line.Seats[s].Hand, hands[s]...)
		line.Seats[s].Cast = cast[s]
		line.Seats[s].Out = outTurn[s]
	}
	placeSeats(&line)
	b, err := json.Marshal(line)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

// gameFacts reads a finished game's log: each seat's in-hand card names,
// its commander casts and the turn count at which it lost (0 if it never
// did).
func gameFacts(eng *rules.Engine, seats int) ([][]string, []int, []int) {
	inHand := map[state.ObjID]bool{}
	seen := map[state.ObjID]bool{}
	started := false
	turns := 0
	cast := make([]int, seats)
	outTurn := make([]int, seats)
	for _, e := range eng.L.Events {
		switch e.Kind {
		case events.TurnChange:
			turns++
			if !started {
				started = true
				for id := range inHand {
					seen[id] = true
				}
			}
		case events.PlayerLost:
			if p := int(e.Player); p >= 0 && p < seats && outTurn[p] == 0 {
				outTurn[p] = max(turns, 1)
			}
		case events.Draw, events.MoveZone:
			if e.To == state.ZHand {
				inHand[e.Obj] = true
				if started && e.From == state.ZLibrary {
					seen[e.Obj] = true
				}
			} else {
				delete(inHand, e.Obj)
			}
		case events.PutOnStack:
			delete(inHand, e.Obj)
			if p := int(e.Player); e.From == state.ZCommand && p >= 0 && p < seats {
				cast[p]++
			}
		}
	}
	hands := make([][]string, seats)
	for id := range seen {
		o := eng.G.Obj(id)
		if o == nil || o.Card == nil || len(o.Card.Faces) == 0 || int(o.Owner) < 0 || int(o.Owner) >= seats {
			continue
		}
		f := o.Card.Faces[0]
		if f.IsBasic() && f.IsLand() {
			continue
		}
		hands[o.Owner] = append(hands[o.Owner], f.Name)
	}
	for s := range hands {
		sort.Strings(hands[s])
	}
	return hands, cast, outTurn
}

// placeSeats fills each seat's finish from the loss turns: the winner is
// first, seats still in a drawn game share the next place, and the losers
// follow, the later loss ahead.
func placeSeats(line *valueLine) {
	for s := range line.Seats {
		vs := &line.Seats[s]
		if s == line.Winner {
			vs.Place = 1
			continue
		}
		if vs.Out == 0 {
			// In at the end of a game it did not win: a draw or stall.
			vs.Place = 1
			if line.Winner >= 0 {
				vs.Place = 2
			}
			continue
		}
		place := 1
		for t := range line.Seats {
			if o := line.Seats[t].Out; t != s && (o == 0 || o > vs.Out) {
				place++
			}
		}
		vs.Place = place
	}
}
