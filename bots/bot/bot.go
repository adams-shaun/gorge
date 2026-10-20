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
		Name: "bot", Label: "Bot", Description: "The production baseline policy. On auto-pay tables it uses combat simulation for attacks and blocks.", Tier: bots.Production,
		Strength: []bots.Measurement{{Claim: "53.17% [52.68, 53.65] constructed and 53.69% [52.73, 54.64] commander vs the plain auto-pay bot", Versus: "plain auto-pay bot", Setting: "held-out auto-pay tables", Source: "commit 66aa546dd"}},
		Cost:     bots.Cost{Scope: "per decision", Note: "not measured; INFERRED <1 ms (bot vs bot ~550k games/h on 8 workers)", Source: "docs/design/2026-09-28-hosted-bot-packages.md §3.1"},
		Formats:  []string{"constructed", "commander"}, Caretaker: "bot",
	}, New: newBot})
}

// Base builds the production bot's policy, unwrapped: the two-ply combat
// simulation for attacks and blocks with auto-pay mana on auto-pay tables, the
// plain heuristic bot otherwise. Policies that layer rules over the production
// bot (forge-heuristic) start from it so they share its combat play.
func Base(o bots.Options) *heuristic.Bot {
	return BaseWithSim(o, botpolicy.DefaultAttackSimParams())
}

// BaseWithSim is Base with caller-supplied combat-sim parameters. The
// parameters apply only when o.AutoPayMana is set; otherwise there is no sim.
func BaseWithSim(o bots.Options, p botpolicy.AttackSimParams) *heuristic.Bot {
	if o.AutoPayMana {
		return heuristic.NewAttackSimBot(o.Seed, p).EnableAutoPayMana()
	}
	return heuristic.NewBot(o.Seed)
}

func newBot(o bots.Options) (seat.Seat, error) {
	s := Base(o)
	if o.CommonSense {
		return sanity.Wrap(s), nil
	}
	return s, nil
}
