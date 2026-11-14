package azredeal

import (
	"errors"
	"fmt"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/seat"
)

// PolicyClosedMaxN is the registry key of the closed-list max^n search: the
// paired bot's closed search (ClosedFactory) with the opponents in its tree,
// each maximising its own value (HostedMaxN's switches). It reads no
// opponent's real list, so it may seat against a human -- a Commander pod's
// bot opponents -- and advise one at a pod table. It has no belief of its own,
// so it is registered only once one is loaded (EnableClosedMaxN; gorged's
// -pod-bot-decks).
const PolicyClosedMaxN = "az-closed-maxn"

// ClosedMaxNSims is the default simulations per decision of PolicyClosedMaxN
// (gorged -pod-bot-sims). Measured on the 4-seat Commander pod
// (TestAZClosedMaxNFourSeatCommanderGame, one game a budget, 11-14), mean and
// p95 ms per searched decision: 32 sims 54/86, 64 sims 105/181, 100 sims
// 183/279; the mulligan ask, on its own budget, peaks near 400 at every
// budget. 64 keeps the p95 an order of magnitude inside the host's 2000 ms
// per-decision budget (server/host/bot_policy.go), and inside it even at the
// 17.7 ms a simulation an older 4-seat measurement on a loaded box reported.
const ClosedMaxNSims = 64

// ClosedMaxNFactory is ClosedFactory with the opponents in the tree and max^n
// at their points (azmcts Options.OpponentNodes and MaxN). At two seats, or
// more than azmcts' max^n seat limit, the search falls back to the paranoid
// read, so a 1v1 table plays ClosedFactory's game.
func ClosedMaxNFactory(belief *searchprobe.Belief, sims int) bots.Factory {
	return func(o bots.Options) (seat.Seat, error) {
		cfg := closedConfig(o, belief, sims)
		cfg.Search.OpponentNodes = true
		cfg.Search.MaxN = true
		return New(o, cfg)
	}
}

// ClosedMaxNInfo is PolicyClosedMaxN's registry declaration at sims
// simulations per decision.
func ClosedMaxNInfo(sims int) bots.Info {
	return bots.Info{
		Name:  PolicyClosedMaxN,
		Label: "az-closed-maxn (search, max^n, closed lists)",
		Description: fmt.Sprintf("The closed-list search for Commander pods: at every searched decision it runs %d MCTS simulations "+
			"over worlds whose opponent cards come from a belief of drafted decks, never their real lists, with each opponent "+
			"maximising its own value (max^n). Plays the production bot everywhere else.", sims),
		Tier: bots.Experimental,
		Cost: bots.Cost{
			Scope:  "per searched decision",
			Note:   "sized by -pod-bot-sims to fit the host's 2000 ms decision budget at a 4-seat pod; contention raises it, and the host cuts a search at the budget and plays the bot's answer",
			Source: "bots/azredeal ClosedMaxNSims",
		},
		Env: true, Search: true, ClosedLists: true,
		// The host's draft events seat a policy only if it plays constructed
		// (eventhost botPolicy), and a constructed table seats it 1v1.
		Formats:   []string{"constructed", "commander"},
		MaxSeats:  4,
		Caretaker: "bot",
	}
}

// closedMaxN is the factory EnableClosedMaxN last armed. The registry entry
// reads it per seat, so a second Enable (another server in one test binary)
// replaces the belief instead of registering twice.
var closedMaxN struct {
	mu         sync.Mutex
	factory    bots.Factory
	registered bool
}

// EnableClosedMaxN registers PolicyClosedMaxN over belief at sims simulations
// per decision. Call it at startup, before the registry is read for the
// offered policies; a later call replaces the belief and sims.
func EnableClosedMaxN(belief *searchprobe.Belief, sims int) error {
	switch {
	case belief == nil:
		return errors.New("azredeal: " + PolicyClosedMaxN + " needs a belief")
	case sims < 1:
		return fmt.Errorf("azredeal: %s: %d simulations, want >= 1", PolicyClosedMaxN, sims)
	}
	closedMaxN.mu.Lock()
	defer closedMaxN.mu.Unlock()
	closedMaxN.factory = ClosedMaxNFactory(belief, sims)
	if closedMaxN.registered {
		return nil
	}
	bots.Register(bots.Entry{Info: ClosedMaxNInfo(sims), New: func(o bots.Options) (seat.Seat, error) {
		closedMaxN.mu.Lock()
		f := closedMaxN.factory
		closedMaxN.mu.Unlock()
		return f(o)
	}})
	closedMaxN.registered = true
	return nil
}
