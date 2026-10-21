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
		Strength: []bots.Measurement{{Claim: "53.17% [52.68, 53.65] constructed and 53.69% [52.73, 54.64] commander vs the plain auto-pay bot. Measured before FORGE-M1 made MulliganForge the default", Versus: "plain auto-pay bot", Setting: "held-out auto-pay tables", Source: "commit 66aa546dd"}, {Claim: "31.5% [27.1, 36.2] vs Forge AI over 400 games (MulliganForge); 28.5% [24.3, 33.1] with the coin mulligan it replaced", Versus: "Forge AI (adams-shaun/forge oracle-harness 82a7d3a440)", Setting: "Forge games over the stdio bridge, 12 Legacy decks, 144 ordered pairs, seats traded", Source: "Forge bridge runs R0m1 and R0v3, 2026-10-20"}},
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
// Both branches answer London mulligans with Forge's keep/bottom rule.
func BaseWithSim(o bots.Options, p botpolicy.AttackSimParams) *heuristic.Bot {
	if o.AutoPayMana {
		return heuristic.NewAttackSimBot(o.Seed, p).EnableAutoPayMana().WithMulligan(botpolicy.MulliganForge)
	}
	return heuristic.NewBot(o.Seed).WithMulligan(botpolicy.MulliganForge)
}

func newBot(o bots.Options) (seat.Seat, error) {
	s := Base(o)
	if o.CommonSense {
		return sanity.Wrap(s), nil
	}
	return s, nil
}
