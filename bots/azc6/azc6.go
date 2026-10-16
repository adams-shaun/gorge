// Package azc6 hosts the az-c6 policy: az-c4's closed-list search seat with
// the C6 network, the net the closed-list self-play loop produced when it was
// restarted from scratch after the 2026-10-16 data loss, trained on 17lands
// FDN limited decks. Which generation is served is the operator's choice
// (cmd/gorged's -az-c6-net).
//
// Everything but the network is az-c5's (and so az-c4's): the search config
// (azc3.ClosedConfig, docs/superpowers/reports/2026-10-15-az-c5.md), the
// London mulligan by land count, the census, the belief, the decks and the
// closed-list contract (bots.Info.ClosedLists; the host checks the seats it
// builds, host/closedlist.go). The implementation is az-c3's
// (bots/azc3.Hosted); only the entry differs.
//
// Like az-c4 it registers only when asked: Enable(path, belief) loads the
// network and registers the policy, and refuses without a belief.
package azc6

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc5"
	"github.com/adams-shaun/gorge/internal/azmcts"
	"github.com/adams-shaun/gorge/internal/searchprobe"
)

// Policy is the registry key.
const Policy = "az-c6"

// report is the source of the inherited search config. The C6 net itself has
// no measurement yet.
const report = "docs/superpowers/reports/2026-10-15-az-c5.md"

var c6 = azc3.NewHosted(azc3.HostedSpec{Info: info(), ErrPrefix: "azc6", Closed: true})

// Enable loads the network at path and registers az-c6 with its opponents'
// lists closed under b. A nil b is refused, as is a path that is not a
// valid .dzgw network; a refusal registers nothing. It must run before the
// server resolves its policy flags, once per process.
func Enable(path string, b *searchprobe.Belief) error { return c6.EnableClosed(path, b) }

// Config is the search config C6 is trained with: az-c5's (az-c4's, which is
// az-c3's closed under b).
func Config(b *searchprobe.Belief) azmcts.SeatConfig { return azc5.Config(b) }

func info() bots.Info {
	return bots.Info{
		Name:  Policy,
		Label: "az-c6 (C6 net, closed-list self-play restarted after the 2026-10-16 data loss; closed decklists)",
		Description: "az-c6: the C6 net. It is az-c4's search seat -- it knows its own list, and at every searched decision it " +
			"guesses yours from 17lands FDN draft decks (the ones whose colours match the cards you have shown), redeals your " +
			"unseen hand and library from that guess, and runs 100 MCTS simulations -- with a network from the closed-list " +
			"self-play loop continued from scratch after the 2026-10-16 data loss, trained on 17lands FDN decks. The served " +
			"generation is the operator's choice. Not yet measured.",
		Tier: bots.Experimental,
		Strength: []bots.Measurement{
			{
				Claim:   "not yet measured: no strength measurement of the C6 net exists",
				Versus:  "sb-tactical",
				Setting: "none yet; the search config is az-c5's (sims=100, closed list, census on)",
				Source:  report,
			},
		},
		Cost: bots.Cost{
			Scope:  "per bot-turn response",
			Note:   "not yet measured for C6; the search config and network shape are az-c5's, so its cost is expected to be similar",
			Source: report,
		},
		Env: true, Search: true, ClosedLists: true,
		Formats:      []string{"constructed"},
		MaxSeats:     2,
		Caretaker:    "bot",
		NeedsAutoPay: true,
	}
}
