// Package forgeheuristic registers the Forge-derived experimental policy.
//
// The policy is the production heuristic bot with the A2 target-selection
// rules armed (LandPriority, ProminentColour, FilterFatalCopies). Those
// rules are clean-room re-expressions of Forge's ComputerUtilCard behaviour
// in gorge's own vocabulary (docs/design/2026-10-19-a2-target-rules.md);
// nothing here is translated from Forge's GPL-3.0 Java. They were measured
// inert on the 30-deck constructed corpus (46.8% A, 95% CI [41.9, 51.6]
// over 400 games, indistinguishable from the bot-vs-bot seat/deck bias), so
// the policy stays opt-in and the default bot is unchanged.
package forgeheuristic

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/seat"
)

// Policy is the registry name of the policy.
const Policy = "forge-heuristic"

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name:        Policy,
		Label:       "Forge heuristic",
		Description: "The production heuristic bot with the measured A2 Forge-derived target rules armed (land colour, prominent colour, fatal-copy filter).",
		Tier:        bots.Experimental,
		Strength: []bots.Measurement{{
			Claim:   "46.8% A [41.9, 51.6] vs bot over 400 games, indistinguishable from the 46.5% bot-vs-bot seat/deck control",
			Versus:  "bot",
			Setting: "constructed, seats traded each game, base seed 1000000",
			Source:  "docs/design/2026-10-19-a2-target-rules.md",
		}},
		Cost:    bots.Cost{Scope: "per decision", Note: "not measured; same policy shape as bot, INFERRED <1 ms", Source: "docs/design/2026-10-19-a2-target-rules.md"},
		Formats: []string{"constructed", "commander"},
	}, New: New})
}

// New builds the policy. The A2 rules are always armed; auto-pay mana is
// enabled exactly when the host asks for it, matching the production bot's
// payment-plan behaviour.
func New(o bots.Options) (seat.Seat, error) {
	b := heuristic.NewBot(o.Seed).EnableTargetRules(botpolicy.TargetRules{
		LandPriority:      true,
		ProminentColour:   true,
		FilterFatalCopies: true,
	})
	if o.AutoPayMana {
		b.EnableAutoPayMana()
	}
	return b, nil
}
