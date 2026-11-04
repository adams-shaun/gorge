// Command gorged serves bot tables, play-vs-bot, spectating and, with -events,
// draft events to a browser: the reference server (server/gorged), and the
// binary the demo deploys (scripts/deploy-demo.sh). This build links gorge's
// built-in hosted bots (bots/all), the experimental search and SpellBench
// policies (bots/search, sbsearch, sbbuiltin, castprofile), the network-backed
// az policies (bots/azc3, azc4, azc5: each registers only when its -<policy>-net
// flag names a network), the draft-event plugin (draft/gorgedevents) and, for
// `gorged review`, the feedback-snapshot loader (review.go). A binary that adds
// extensions (more bots) wraps gorged.Main the same way with its own
// gorged.Extensions.
package main

import (
	"os"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/bots/azc3"
	"github.com/adams-shaun/gorge/bots/azc4"
	"github.com/adams-shaun/gorge/bots/azc5"
	_ "github.com/adams-shaun/gorge/bots/castprofile"
	_ "github.com/adams-shaun/gorge/bots/sbbuiltin"
	_ "github.com/adams-shaun/gorge/bots/sbsearch"
	_ "github.com/adams-shaun/gorge/bots/search"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/draft/gorgedevents"
	"github.com/adams-shaun/gorge/server/gorged"
)

// netBots are the network policies gorged's flags enable
// (docs/design/2026-11-03-hosted-az-policies.md). Registration order is the
// order of their reserved pools and demo tables.
var netBots = []struct {
	policy string
	bot    gorged.NetBot
}{
	{azc3.Policy, gorged.NetBot{
		NetHelp:        "its .dzgw network",
		ReadsDecklists: true,
		Enable: func(path string, belief *searchprobe.Belief) error {
			if err := azc3.Enable(path); err != nil {
				return err
			}
			azc3.SetBelief(belief)
			return nil
		},
	}},
	{azc4.Policy, gorged.NetBot{NetHelp: "its .dzgw network", Enable: azc4.Enable}},
	{azc5.Policy, gorged.NetBot{NetHelp: "its .dzgw network (the C5 net, trained after the payment-plan fixes)", Enable: azc5.Enable}},
}

func init() {
	for _, nb := range netBots {
		gorged.RegisterNetBot(nb.policy, nb.bot)
	}
}

func extensions() gorged.Extensions {
	return gorged.Extensions{Review: reviewLoad, Events: gorgedevents.Plugin{}}
}

func main() { os.Exit(gorged.Main(os.Args[1:], extensions())) }
