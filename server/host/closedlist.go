package host

import (
	"fmt"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
)

// Decklist privacy at a table with a human (operator ruling 2026-10-13: an
// opponent's decklist is private information).
//
// Table validation refuses, by name, every policy that reads decklists
// (bots.Info.ReadsDecklists: Env without ClosedLists). A closed-list policy
// (bots.Info.ClosedLists) passes validation on its declaration, and
// these two checks make the server, not the declaration, the guarantee:
//
//   - closedListGuard runs once per match, after the seats are final: every
//     non-human seat that would get a search Env (bots.EnvSeat) must be a
//     searchseat.ClosedListSeat whose SearchBelief is non-nil. Otherwise the
//     match crashes before its first decision -- nothing of the opponent's
//     list was ever handed out.
//   - envData re-checks at every decision (closedSetup): the Setup it hands
//     a seat at a human table must be closed (searchseat.SeatSetup gave it a
//     belief). If not, the seat gets its own list only and no root.
//
// What "closed" buys: the seat's Setup holds only
// its own list; its honest root, and every world its search deals from that
// root, re-identify the opponent's unknown hand and library cards from the
// belief and carry only the seat's own deck manifest
// (rules.Engine.ClosedDeckManifests).

// closedListGuard refuses a match at a table with a human whose seats
// include an Env seat with open opponent lists. A bot-only table is left to
// validation (AllowDecklistBots).
func closedListGuard(c TableConfig, seats []seat.Seat) error {
	if len(c.Humans) == 0 {
		return nil
	}
	for i, s := range seats {
		if _, ok := s.(*HumanSeat); ok {
			continue
		}
		if _, ok := s.(bots.EnvSeat); !ok {
			continue
		}
		cl, ok := s.(searchseat.ClosedListSeat)
		if !ok || cl.SearchBelief() == nil {
			return fmt.Errorf("host: table %s: seat %d (%T) would search with its opponents' real decklists at a table with a human; only a closed-list seat (searchseat.ClosedListSeat with a belief) may", c.ID, i, s)
		}
	}
	return nil
}

// openListRefusal is envData's refusal reason for an open Setup at a table
// with a human.
const openListRefusal = "decklist privacy: an open-list search seat at a table with a human"

// posedRefusal is envData's refusal reason for a decision posed inside a
// tape resolution at a table with a human. Such a decision's engine holds
// the resolution's checkpoint S0 -- the live game as the resolution began,
// every hidden card real -- and every clone of it, the honest root and each
// world the search deals from it, shares that checkpoint and re-runs the
// resolution from it when answered. The closed redeal re-identifies the
// opponent's cards at the posed point, not in S0, so a root built there
// would carry the opponent's real hidden cards. The seat plays its
// non-searched fallback instead; it costs a searched decision, a small
// fraction of a game's (the leak audit counts them).
const posedRefusal = "decklist privacy: a decision posed inside a resolution (its checkpoint holds the real hidden cards)"

// closedRootRefusal is envData's per-decision gate on top of closedSetup: at
// a table with a human, no root is built at a posed tape resolution
// (posedRefusal). "" lets the root be built.
func closedRootRefusal(c TableConfig, e *rules.Engine) string {
	if len(c.Humans) > 0 && rules.TapePosed(e) {
		return posedRefusal
	}
	return ""
}

// closedSetup is envData's per-decision check: at a table with a human, a
// Setup that SeatSetup left open (no belief) is cut to the actor's own list
// and refused. closedListGuard makes this unreachable; it is the fail-closed
// second line, so no future seat type or wrapper that slips past the start
// check is ever handed an opponent's list. ok false means: no root.
func closedSetup(c TableConfig, setup searchprobe.PublicGame, p state.PlayerID) (searchprobe.PublicGame, bool) {
	if len(c.Humans) == 0 || setup.Belief != nil {
		return setup, true
	}
	own := make([][]*cards.Card, len(setup.Decks))
	if int(p) >= 0 && int(p) < len(own) {
		own[p] = setup.Decks[p]
	}
	setup.Decks = own
	return setup, false
}
