// Package forgeheuristic registers the Forge-derived experimental policy.
//
// The policy is the production heuristic bot with the A2 target-selection
// rules armed (LandPriority, ProminentColour, FilterFatalCopies), the A1
// removal-now rule (HoldForBigger), the A6 timing rules (CounterTop,
// DrawTokenLate and PumpInCombat), the A3 Forge AI-hint rules (PlayMain1
// timing and the RemoveDeck cast veto), the A4 deck-shape posture (R2 aggro
// releases the held blocker, R3 attack-sim life weights, R4 reserve scale) and
// the A7 combat rules (gang blocks, a gang-aware predicted reply and Double
// Strike in blocked combat, all inside the combat simulation). Those rules are
// clean-room re-expressions of Forge's
// ComputerUtilCard behaviour in gorge's own vocabulary
// (docs/design/2026-10-19-a2-target-rules.md,
// docs/design/2026-10-20-a1-removal-now.md,
// docs/design/2026-10-20-a6-timing-rules.md and
// docs/design/2026-10-20-a3-svar-hints.md); nothing here is translated from
// Forge's GPL-3.0 Java. A2 was measured inert on the 30-deck constructed
// corpus (46.8% A, 95% CI [41.9, 51.6] over 400 games, indistinguishable from
// the bot-vs-bot seat/deck bias), so the policy stays opt-in and the default
// bot is unchanged. A1's Strength claim has not been re-measured.
package forgeheuristic

import (
	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/bot"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/sanity"
	"github.com/adams-shaun/gorge/engine/seat"
)

// Policy is the registry name of the policy.
const Policy = "forge-heuristic"

// HoldPolicy is the registry name of the same policy with the A6
// instant-speed hold rule forced on, so a bench can A/B it by policy name.
const HoldPolicy = "forge-heuristic-hold"

// OptHoldInstant is the declared option that arms the A6 instant-speed hold
// rule on Policy.
const OptHoldInstant = "hold-instant"

const holdInstantHelp = "arm the A6 instant-speed hold rule (hold instants outside the respond / opponent post-combat / combat-trick windows)"

func init() {
	bots.Register(bots.Entry{Info: bots.Info{
		Name:        Policy,
		Label:       "Forge heuristic",
		Description: "The production bot (combat simulation, auto-pay mana, common-sense wrap) plus the Forge rules: the measured A2 Forge-derived target rules armed (land colour, prominent colour, fatal-copy filter), the A1 removal-now hold rule, the A6 counter, draw/token and pump-in-combat timing rules, the A3 Forge AI hints (PlayMain1 timing, RemoveDeck veto), the A5 attach rules (aura polarity, carrier choice, boon-Aura hold), the A4 deck-shape posture (aggro releases the last blocker, sim life weights, reserve scale), the A7 combat rules (gang blocks, gang-aware predicted reply, Double Strike in blocked combat) and Forge's London mulligan (keep/mulligan and bottoming).",
		Tier:        bots.Experimental,
		Strength: []bots.Measurement{{
			Claim:   "46.8% A [41.9, 51.6] vs bot over 400 games, indistinguishable from the 46.5% bot-vs-bot seat/deck control; measured before F0: no attack sim on auto-pay tables. Measured before FORGE-A8 armed the forge mulligan.",
			Versus:  "bot",
			Setting: "constructed, seats traded each game, base seed 1000000",
			Source:  "docs/design/2026-10-19-a2-target-rules.md",
		}, {
			Claim:   "32.0% [27.6, 36.7] vs Forge AI over 400 games, +0.5 pp over bot with the same mulligan (inside noise)",
			Versus:  "Forge AI (adams-shaun/forge oracle-harness 82a7d3a440)",
			Setting: "Forge games over the stdio bridge, 12 Legacy decks, 144 ordered pairs, seats traded",
			Source:  "Forge bridge run R1v3, 2026-10-20",
		}},
		Cost:    bots.Cost{Scope: "per decision", Note: "not measured; same policy shape as bot, INFERRED <1 ms", Source: "docs/design/2026-10-19-a2-target-rules.md"},
		Formats: []string{"constructed", "commander"},
		Options: []bots.OptionSpec{{
			Key:     OptHoldInstant,
			Type:    "bool",
			Default: "false",
			Help:    holdInstantHelp,
		}},
	}, New: New})
	bots.Register(bots.Entry{Info: bots.Info{
		Name:        HoldPolicy,
		Label:       "Forge heuristic + hold",
		Description: "Forge heuristic with the A6 instant-speed hold rule armed: it holds instants outside the respond, opponent post-combat and combat-trick windows.",
		Tier:        bots.Experimental,
		Strength: []bots.Measurement{{
			Claim:  "not measured",
			Source: "registered 2026-10-21 without a measurement",
		}},
		Cost:    bots.Cost{Scope: "per decision", Note: "not measured; same policy shape as forge-heuristic", Source: "registered 2026-10-21 without a measurement"},
		Formats: []string{"constructed", "commander"},
	}, New: func(o bots.Options) (seat.Seat, error) {
		if o.Config == nil {
			o.Config = map[string]string{}
		}
		o.Config[OptHoldInstant] = "true"
		return New(o)
	}})
}

// simParams is the production combat simulation's parameters with the A7
// combat rules armed.
func simParams() botpolicy.AttackSimParams {
	p := botpolicy.DefaultAttackSimParams()
	p.Combat = botpolicy.CombatRules{GangBlocks: true, GangPredict: true, DoubleStrike: true}
	return p
}

// holdInstant reads the declared option, defaulting to off when New is called
// directly with a Config that does not carry it (bots.New always supplies the
// declared default).
func holdInstant(o bots.Options) bool {
	if _, ok := o.Config[OptHoldInstant]; !ok {
		return false
	}
	return o.Bool(OptHoldInstant)
}

// New builds the policy: the production bot's constructor (bot.BaseWithSim, so
// the combat simulation and auto-pay mana come as they do for bot, plus the A7
// combat rules) with the Forge rules armed, wrapped by the common-sense filter
// when the host asks.
func New(o bots.Options) (seat.Seat, error) {
	b := bot.BaseWithSim(o, simParams()).EnableTargetRules(botpolicy.TargetRules{
		LandPriority:      true,
		ProminentColour:   true,
		FilterFatalCopies: true,
	}).EnableRemovalRules(botpolicy.RemovalRules{
		HoldForBigger: true,
	}).EnableTimingRules(botpolicy.TimingRules{
		CounterTop:    true,
		DrawTokenLate: true,
		PumpInCombat:  true,
		HoldInstant:   holdInstant(o),
	}).EnableAIHints(botpolicy.AIHints{
		Enabled: true,
	}).EnableAttachRules(botpolicy.AttachRules{
		AuraPolarity: true,
		Carrier:      true,
		HoldOwnAura:  true,
	}).EnablePostureRules(botpolicy.PostureRules{
		Attack:  true,
		Reserve: true,
	}).WithMulligan(botpolicy.MulliganForge)
	if o.CommonSense {
		return sanity.WrapMulligan(b, botpolicy.MulliganForge), nil
	}
	return b, nil
}
