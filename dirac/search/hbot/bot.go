// Package hbot is gorge's heuristic bot with auto-pay mana
// (bots/heuristic.Bot.EnableAutoPayMana over bots/botpolicy.Decide, the
// bot bots/azmcts's autopay walk asks for both seats) ported natively onto
// kernel state: the kernel search's Bot (dirac/search.Bot), answering
// every decision the tree does not search and supplying candidate 0 at
// every searched point.
//
// It imports only the kernel. The board (board.go) is botpolicy.Board read
// off the State through its accessors into fixed arrays; per-card facts
// gorge's bot reads off strings (mana production, the counter-spell API,
// an ability label's cost, a targeted SA's effect) come from Facts, built
// once per table (dirac/oracle.HBotFacts). No maps, no strings, no
// allocation per call.
//
// Each decision kind follows gorge's rule set and gorge's option order
// (every gorge tie breaks on option index), over the kernel options gorge's
// bot would see:
//
//   - priority (priority.go): the auto-pay wrapper's private list -- the
//     land plays, the pool-paid casts, the "ability" options, then one
//     plan-backed cast per payable object -- ranked by chooseLand,
//     chooseCast (C1-C8 with the reserve), chooseAbility (A1-A6), else
//     pass. A kernel cast gorge's planner does not price (a kicked cast the
//     pool cannot pay, an {X} spell) is invisible to it, except through
//     the manual path (wantsManual), which taps mana for the card
//     Board.TapIntent names: the kernel has no manual mana, so the port
//     plays the taps out on its board and ranks what the floated pool pays.
//   - attackers / blockers (combat.go): the whole declaration
//     (chooseAttackersMode with AR7 lethal pressure, LegalAttackChoices;
//     chooseBlockers, legalBlockChoices) recomputed at every single pick
//     from the declaration's original option set, served one pick at a
//     time, Done last.
//   - targets (target.go): chooseTargets with gorge's rankers.
//   - choose / modes / optional / trigger order / mulligan (choose.go).
package hbot

import (
	"github.com/adams-shaun/gorge/dirac"
)

// Bot is the port. It is not safe for concurrent use (scratch board).
type Bot struct {
	f *Facts
	b board

	// hold is gorge's TimingRules.HoldInstant; zero = off.
	hold bool
	// fetchNeed is the fetchneed option (fetch.go); zero = off.
	fetchNeed bool

	// priority scratch
	pc      [dirac.MaxOptions]cand
	payable [dirac.MaxObj]dirac.Obj
	// pvC / ppC memoise plannedVisible / FeatPoolPays per option of the
	// current priority decision: 0 unknown, 1 false, 2 true.
	pvC, ppC [dirac.MaxOptions]uint8
	// floated is the mana floatByHand has added to the board's pool.
	floated [6]int32
	// target scratch
	tr [dirac.MaxOptions]tgtRank
}

// New returns a bot over f (NewFacts or dirac/oracle.HBotFacts).
func New(f *Facts) *Bot { return &Bot{f: f} }

// EnableHoldInstant arms gorge's TimingRules.HoldInstant: the bot holds an
// instant-speed cast outside the windows the rule allows. The zero value is
// off, so every existing bot is unchanged.
func (bt *Bot) EnableHoldInstant() *Bot { bt.hold = true; return bt }

// HoldInstant reports whether the hold rule is armed.
func (bt *Bot) HoldInstant() bool { return bt.hold }

// PurePick marks the bot pure (dirac/search.PureBot): Pick reads only the
// state and decision it is given and refills its board from them, so
// skipping a call cannot change any later answer.
func (*Bot) PurePick() {}

// Pick answers d (an index into d.Options[:d.N]); dirac/search.Bot.
func (bt *Bot) Pick(s *dirac.State, d *dirac.Decision) int {
	if d.N <= 1 {
		return 0
	}
	switch d.Kind {
	case dirac.DkPriority:
		return bt.priority(s, d)
	case dirac.DkAttackers:
		return bt.attackers(s, d)
	case dirac.DkBlockers:
		return bt.blockers(s, d)
	case dirac.DkTarget:
		return bt.target(s, d)
	case dirac.DkModes:
		return bt.modes(s, d)
	case dirac.DkChoose:
		return bt.choose(s, d)
	case dirac.DkTriggerOrder:
		return bt.triggerOrder(s, d)
	case dirac.DkMulligan:
		return bt.mulligan(s, d)
	}
	return 0
}

// index helpers

func passIndex(d *dirac.Decision) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPass {
			return i
		}
	}
	return 0
}

func doneIndex(d *dirac.Decision) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			return i
		}
	}
	return -1
}

func optIndex(d *dirac.Decision, a dirac.Action) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == a {
			return i
		}
	}
	return -1
}

// Facts is the facts the bot reads.
func (bt *Bot) Facts() *Facts { return bt.f }
