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
		Strength: []bots.Measurement{{Claim: "53.17% [52.68, 53.65] constructed and 53.69% [52.73, 54.64] commander vs the plain auto-pay bot. Measured before FORGE-M1 made MulliganForge the default", Versus: "plain auto-pay bot", Setting: "held-out auto-pay tables", Source: "commit 66aa546dd"}, {Claim: "31.5% [27.1, 36.2] vs Forge AI over 400 games (MulliganForge); 28.5% [24.3, 33.1] with the coin mulligan it replaced", Versus: "Forge AI (adams-shaun/forge oracle-harness 82a7d3a440)", Setting: "Forge games over the stdio bridge, 12 Legacy decks, 144 ordered pairs, seats traded", Source: "Forge bridge runs R0m1 and R0v3, 2026-10-20"}, {Claim: "51.64% vs a 50.21% bot-vs-bot control over 7,680 games per arm (five seeds 1/7/42/99/123 x 12 suite repetitions, 5/5 seeds up); +1.24pp against forge-heuristic (51.38% vs a 50.14% same-seed control) with PumpInCombat armed and no other timing rule", Versus: "bot-vs-bot control and forge-heuristic", Setting: "FDN Limited suite, five seeds x 12 repetitions, 7,680 games per arm", Source: "docs/design/2026-10-22-fdn-playstyle-heuristic.md, \"THE RESULT\""}},
		Cost:     bots.Cost{Scope: "per decision", Note: "not measured; INFERRED <1 ms (bot vs bot ~550k games/h on 8 workers)", Source: "docs/design/2026-09-28-hosted-bot-packages.md §3.1"},
		Formats:  []string{"constructed", "commander"}, Caretaker: "bot",
	}, New: newBot})
}

// timingRules is the A6 timing rule set the production bot plays. ONLY
// PumpInCombat is armed, deliberately:
//
//   - PumpInCombat is worth +1.43pp on the FDN Limited suite (five seeds x 12
//     repetitions, 7,680 games per arm, 51.64% vs a 50.21% bot-vs-bot control,
//     5/5 seeds up) and +1.24pp against forge-heuristic. It restricts an
//     instant creature pump to declare-blockers and only to a pump that saves
//     a combatant, lets one kill a survivor, or makes an unblocked attack
//     lethal.
//   - HoldInstant is NOT armed: alone it measured 49.31% (0/5 seeds up), the
//     worst configuration tried, because it holds instants the bot can no
//     longer cast later. Adding it would be a regression.
//   - CounterTop (50.55%) and DrawTokenLate (50.12%) are neutral alone, and all
//     three together gave 51.55%, slightly worse than PumpInCombat alone.
//
// See docs/design/2026-10-22-fdn-playstyle-heuristic.md, "THE RESULT".
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
