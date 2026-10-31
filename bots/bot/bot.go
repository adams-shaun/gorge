package bot

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/sanity"
	"github.com/adams-shaun/gorge/engine/seat"
)

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name: "bot", Label: "Bot", Description: "The production baseline policy. On auto-pay tables it uses combat simulation for attacks and blocks. London mulligan: Forge's keep/bottom rule (MulliganForge).", Tier: bots.Production,
		Cost:    bots.Cost{Scope: "per decision", Note: "not measured; under 1 ms by inference", Source: "docs/design/2026-09-28-hosted-bot-packages.md §3.1"},
		Formats: []string{"constructed", "commander"}, Caretaker: "bot",
	}, New: newBot})
}

// timingRules is the timing rule set the production bot plays. Only
// PumpInCombat is armed: it restricts an instant creature pump to
// declare-blockers and to a pump that saves a combatant, lets one kill a
// survivor, or makes an unblocked attack lethal. HoldInstant alone plays worse,
// because it holds instants the bot can no longer cast later; CounterTop and
// DrawTokenLate add nothing. pump_in_combat_default_test.go pins the default.
var timingRules = botpolicy.TimingRules{PumpInCombat: true}

// Base builds the production bot's policy, unwrapped: the two-ply combat
// simulation for attacks and blocks with auto-pay mana on auto-pay tables, the
// plain heuristic bot otherwise. Policies that layer rules over the production
// bot (forge-heuristic) start from it so they share its combat play.
func Base(o bots.Options) *heuristic.Bot {
	return BaseWithSim(o, botpolicy.DefaultAttackSimParams())
}

// BaseWithSim is Base with caller-supplied combat-sim parameters. The
// parameters apply only when o.AutoPayMana is set; otherwise there is no sim.
// Both branches answer London mulligans with Forge's keep/bottom rule.
func BaseWithSim(o bots.Options, p botpolicy.AttackSimParams) *heuristic.Bot {
	if o.AutoPayMana {
		return heuristic.NewAttackSimBot(o.Seed, p).EnableAutoPayMana().
			WithMulligan(botpolicy.MulliganForge).EnableTimingRules(timingRules)
	}
	return heuristic.NewBot(o.Seed).WithMulligan(botpolicy.MulliganForge).
		EnableTimingRules(timingRules)
}

func newBot(o bots.Options) (seat.Seat, error) {
	s := Base(o)
	if o.CommonSense {
		return sanity.Wrap(s), nil
	}
	return s, nil
}
