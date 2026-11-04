// Package azc3 hosts the az-c3 policy: the az-redeal search seat with the C3
// network of the az self-play loop in place of the heuristic prior and leaf
// (docs/design/2026-11-03-hosted-az-policies.md).
//
// It is the search seat exactly as the loop trained and measured it, honest
// worlds throughout:
//
//	az:sims=100:autopay:oppnodes:oppfull:morekinds:topk=6:gonet=c3.dzgw
//
// with the London mulligan by land count (botpolicy.MulliganLands). The
// net is the in-process mlp (bots/gonet) read from a .dzgw file, shared by
// every table in the process.
//
// Unlike every other entry it registers only when asked: Enable(path) loads
// the network and registers the policy, and gorged calls it for -az-c3-net.
// Linking the package registers nothing, because gorged offers every
// registered entry by default and an offered policy with no network behind it
// would fail at table creation. A network is a deployment artifact, not a
// source file.
//
// Trained on one distribution. The net saw only 17lands FDN limited decks
// (31,516 of them); it has never seen a constructed deck, and what it does
// there is unmeasured. The server keeps the policy to the decks it was
// trained on: see the deck pool in cmd/gorged.
package azc3

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azmcts"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
)

// Policy is the registry key.
const Policy = "az-c3"

// c3 is az-c3's hosted registration: open lists unless SetBelief closes them.
var c3 = NewHosted(HostedSpec{Info: info(), ErrPrefix: "azc3"})

// SetBelief closes the opponents' deck lists for every az-c3 seat built
// afterwards: the search deals each opponent's unknown cards from lists
// drawn from b (azmcts.WorldRedealClosed) instead of the opponent's real
// list. gorged calls it for -az-c3-belief. Nil restores the open lists.
func SetBelief(b *searchprobe.Belief) { c3.SetBelief(b) }

// Enable loads the network at path and registers the az-c3 policy. It must run
// before the server resolves its policy flags, once per process; a path that
// is not a valid .dzgw network is an error and registers nothing.
func Enable(path string) error { return c3.Enable(path) }

// Info is the registry entry Enable registers, for a caller that describes
// the policy without enabling it (cmd/gorged's docs check).
func Info() bots.Info { return info() }

func info() bots.Info {
	return bots.Info{
		Name:  Policy,
		Label: "az-c3 (AlphaZero search + trained net)",
		Description: "The az search seat in its honest world with a small network trained on its own self-play: " +
			"at every searched decision it runs 100 MCTS simulations over freshly redealt worlds rebuilt from its own " +
			"observations, the net ranking the candidates and scoring the leaves, and the opponent modelled as a " +
			"searching player. Trained and measured on 17lands FDN limited decks only; play it on those.",
		Tier: bots.Experimental,
		Cost: bots.Cost{
			MeanMS: 185, P95MS: 259, Scope: "per bot-turn response",
			Note: "hosted smoke: 4 games against a human who only passes, the 32 waits that included bot search (a small sample); " +
				"the training loop's evaluations measured 202-344 ms mean per searched decision with 24 workers in parallel",
			Source: "docs/design/2026-11-03-hosted-az-policies.md",
		},
		Env: true, Search: true,
		Formats:      []string{"constructed"},
		MaxSeats:     2,
		Caretaker:    "bot",
		NeedsAutoPay: true,
	}
}

// Config is the search config the c3 net was trained and measured with:
// azredeal.Hosted (honest redeal world, no clairvoyant source) plus the keys
// of the spec in the package comment, the keys the training loop's policy
// spec maps onto the same fields; TestConfigMatchesTheMeasuredSpec pins them.
func Config() azmcts.SeatConfig {
	cfg := azredeal.Hosted()
	cfg.Search.Kinds = azmcts.FullKinds() // :morekinds
	cfg.Search.AutoPayment = true         // :autopay
	cfg.Search.OpponentNodes = true       // :oppnodes
	cfg.Search.OpponentMirror = true      // :oppfull
	cfg.Search.PriorTopK = 6              // :topk=6
	cfg.Mulligan = botpolicy.MulliganLands
	return cfg
}

// ClosedConfig is Config with the opponents' lists closed under b
// (azmcts.WorldRedealClosed): the seat's own list stays known, the
// opponent's unknown cards are dealt from lists drawn from b. A nil b is
// Config, the open lists.
func ClosedConfig(b *searchprobe.Belief) azmcts.SeatConfig {
	cfg := Config()
	if b != nil {
		cfg.World, cfg.Belief = azmcts.WorldRedealClosed, b
	}
	return cfg
}
