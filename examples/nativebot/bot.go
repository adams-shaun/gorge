// Command nativebot is the smallest example of a bot policy linked into a
// gorge server. It answers every decision with the first legal option and
// declares one configuration option, so it also demonstrates the
// -bot-opt/-bot-config surface.
//
// Run it with:
//
//	go run ./examples/nativebot -bot-opt nativebot.greeting=hello -tables 1 -seats 2
package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// Name is the policy name gorged's -bot-policy, -bot-policies and -bot-opt
// flags use.
const Name = "nativebot"

func init() {
	bots.Register(bots.Entry{
		Info: bots.Info{
			// Name is the registry key every flag and API uses.
			Name: Name,
			// Label and Description are the human-facing strings the server
			// can show in a policy listing.
			Label:       "Native example",
			Description: "answers every decision with the first legal option; demonstrates a linked-in policy with a declared option",
			// Tier is Production or Experimental; Register rejects anything
			// else. An example policy is Experimental.
			Tier: bots.Experimental,
			// Strength and Cost are optional measurement metadata. The zero
			// values mean "not measured", which is honest for an example.
			Strength: nil,
			Cost:     bots.Cost{},
			// Env must stay false: an Env policy builds a search engine over
			// every seat's REAL decklist. The host refuses one at a table
			// with a human and requires AllowDecklistBots on a bot-only
			// table. This seat reads only the view it is handed.
			Env: false,
			// Search marks a policy whose decisions are searched; the host
			// bounds how many run concurrently. This seat never searches.
			Search: false,
			// ClosedLists is only meaningful with Env. It declares that an
			// Env seat never opens an opponent's list; Register rejects
			// ClosedLists without Env.
			ClosedLists: false,
			// Formats lists the table formats the policy may play. gorged's
			// -bot-policy validation requires both for the default, and the
			// example is deliberately playable in either.
			Formats: []string{"constructed", "commander"},
			// MaxSeats bounds the table size this policy accepts. The example
			// is a 1v1 policy; 0 would mean no bound.
			MaxSeats: 2,
			// Caretaker names a policy the host may build when a human seat
			// times out. Empty means this policy is not offered as a
			// caretaker; the host falls back to the table's own policy.
			Caretaker: "",
			// NeedsAutoPay marks a policy with no manual-tap mode, so the
			// host refuses a table that does not publish payment plans. This
			// seat answers whatever options it is offered and needs neither.
			NeedsAutoPay: false,
			// Options are the keys the factory reads. bots.New fills every
			// Default into Options.Config before calling the factory, and
			// gorged validates -bot-opt/-bot-config/the environment against
			// these declarations at startup.
			Options: []bots.OptionSpec{{
				Key:     "greeting",
				Type:    "string",
				Default: "hello from nativebot",
				Help:    "text logged once per game",
			}},
		},
		New: newNativeBot,
	})
}

func newNativeBot(o bots.Options) (seat.Seat, error) {
	return &nativeBot{greeting: o.String("greeting")}, nil
}

type nativeBot struct {
	greeting string
	once     sync.Once
}

// Decide picks the first legal option (or the first Min of them). It is
// deliberately rules-ignorant: Decision.Validate is the authority on what is
// legal, and the engine rejects anything this seat gets wrong.
func (b *nativeBot) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	b.once.Do(func() {
		fmt.Fprintf(os.Stderr, "nativebot: %s\n", b.greeting)
	})
	n := d.Min
	if n < 0 {
		n = 0
	}
	if n > len(d.Options) {
		n = len(d.Options)
	}
	choices := make([]int, n)
	for i := range choices {
		choices[i] = i
	}
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}
	if err := d.Validate(in); err != nil {
		return decision.Intent{}, fmt.Errorf("nativebot: cannot answer %s: %w", d.Kind, err)
	}
	return in, nil
}
