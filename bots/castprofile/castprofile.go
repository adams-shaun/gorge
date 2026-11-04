// Package castprofile hosts the cast-profile policy: the production bot
// playing from a cast profile. The embedded default profile is
// intent-identical to the manual bot, so the entry exists to serve a fitted
// profile beside it.
package castprofile

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/seat"
)

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name: "cast-profile", Label: "Cast Profile", Description: "Plays the production bot with a cast profile; the embedded default profile is intent-identical to the manual bot.", Tier: bots.Experimental,
		Cost:    bots.Cost{Scope: "per decision", Note: "as bot; fitted profile is not embedded", Source: "docs/design/2026-09-28-hosted-bot-packages.md §3.1"},
		Formats: []string{"constructed", "commander"}, Caretaker: "bot",
	}, New: newCastProfile})
}

func newCastProfile(o bots.Options) (seat.Seat, error) {
	b, err := heuristic.NewCastProfileBot(o.Seed)
	if err != nil {
		return nil, err
	}
	if o.AutoPayMana {
		b.EnableAutoPayMana()
	}
	// the production bot arms the PumpInCombat timing rule by default
	// (bots/bot's timing rules, docs/design/2026-10-20-a6-timing-rules.md);
	// the default profile follows it so it stays intent-identical to the bot
	b.EnableTimingRules(botpolicy.TimingRules{PumpInCombat: true})
	return b, nil
}
