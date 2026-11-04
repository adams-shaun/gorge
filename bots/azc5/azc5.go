// Package azc5 hosts the az-c5 policy: az-c4's closed-list search seat with
// the C5 network (docs/design/2026-11-03-hosted-az-policies.md), the net
// az-c4's closed-list, census-on loop produced when it was continued for four
// generations on the build carrying the payment-plan fixes (triggered-ability
// payments, bots consuming payment plans, the manual tap gate):
//
//	az:sims=100:autopay:oppnodes:oppfull:morekinds:closedlist:topk=6:gonet=c5.dzgw
//
// Everything but the network is az-c4's: the search config (azc3.ClosedConfig),
// the London mulligan by land count, the census, the belief, the decks and
// the closed-list contract (bots.Info.ClosedLists; the host checks the seats
// it builds, server/host/closedlist.go). The implementation is az-c3's
// (bots/azc3.Hosted); only the entry differs.
//
// Like az-c4 it registers only when asked: Enable(path, belief) loads the
// network and registers the policy, and refuses without a belief. The net
// saw only 17lands FDN limited decks; the server keeps it to them
// (cmd/gorged's -az-c5-decks pool).
package azc5

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/searchprobe"
)

// Policy is the registry key.
const Policy = "az-c5"

// report is the source every claim below cites.
const report = "docs/design/2026-11-03-hosted-az-policies.md"

var c5 = azc3.NewHosted(azc3.HostedSpec{Info: info(), ErrPrefix: "azc5", Closed: true})

// Enable loads the network at path and registers az-c5 with its opponents'
// lists closed under b. A nil b is refused, as is a path that is not a
// valid .dzgw network; a refusal registers nothing. It must run before the
// server resolves its policy flags, once per process.
func Enable(path string, b *searchprobe.Belief) error { return c5.EnableClosed(path, b) }

// Config is the search config C5 was trained and measured with: az-c4's,
// which is az-c3's (azc3.Config) closed under b.
func Config(b *searchprobe.Belief) azmcts.SeatConfig { return azc3.ClosedConfig(b) }

// Info is the registry entry Enable registers, for a caller that describes
// the policy without enabling it (cmd/gorged's docs check).
func Info() bots.Info { return info() }

func info() bots.Info {
	return bots.Info{
		Name:  Policy,
		Label: "az-c5 (C5 net, trained after the payment-plan fixes; closed decklists)",
		Description: "az-c5: the C5 net, trained after the payment-plan fixes. It is az-c4's search seat -- it knows its own list, " +
			"and at every searched decision it guesses yours from 28,366 17lands FDN draft decks (the ones whose colours match " +
			"the cards you have shown), redeals your unseen hand and library from that guess, and runs 100 MCTS simulations -- " +
			"with a network that continued az-c4's self-play training for four more generations on the engine with the " +
			"payment-plan fixes in. Trained and measured on 17lands FDN limited decks only; it plays only those.",
		Tier: bots.Experimental,
		Cost: bots.Cost{
			MeanMS: 97, P95MS: 173, Scope: "per bot-turn response",
			Note: "hosted smoke: 5 games against a human who only passes, one game at a time, with the az-c5 vs az-c5 demo table " +
				"playing on the same server (4 CPUs); the 266 waits of 20 ms or more (the ones carrying a bot search; max 266 ms). " +
				"All 619 waits that included a bot decision: mean 43 ms, p95 131. The training loop measured 114-117 ms mean per " +
				"searched decision with 24 games in parallel",
			Source: report,
		},
		Env: true, Search: true, ClosedLists: true,
		Formats:      []string{"constructed"},
		MaxSeats:     2,
		Caretaker:    "bot",
		NeedsAutoPay: true,
	}
}
