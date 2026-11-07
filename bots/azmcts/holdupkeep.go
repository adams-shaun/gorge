package azmcts

import (
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// upkeepHold is Options.HoldUpkeep armed on a search's enumeration scratch
// (boardScratch.hold): the rule botpolicy.Board.PrematureInstantCast states,
// applied to the priority candidates at the root and in the walk.
type upkeepHold struct {
	// exempt is Options.HoldUpkeepExempt: card names never held.
	exempt []string
	// drops counts held casts dropped from a candidate list; Search reads
	// it after the root's enumeration (Stats.HeldUpkeep).
	drops int
}

// holdOn reports whether s arms the hold and decision d of e sits in its
// window: a priority decision of the active player in its untap, upkeep or
// draw step with an empty stack. It is the cheap pre-check over the engine's
// own facts; the board predicate decides each option.
func (s *boardScratch) holdOn(e *rules.Engine, d *decision.Decision) bool {
	if s == nil || s.hold == nil || e == nil || d == nil || d.Kind != decision.KPriority {
		return false
	}
	g := e.G
	if g.Active != d.Player || len(g.Stack) != 0 {
		return false
	}
	switch g.Step {
	case state.StepUntap, state.StepUpkeep, state.StepDraw:
		return true
	}
	return false
}

// holds reports whether option o, on board b of e, is a held cast, and
// counts it in s.hold.drops when it is (the caller drops it).
func (s *boardScratch) holds(e *rules.Engine, b *botpolicy.Board, o decision.Option) bool {
	if !b.HeldUpkeepCast(e.G, o, s.hold.exempt) {
		return false
	}
	s.hold.drops++
	return true
}

// heldAnswer is the intent the hold plays in place of the bot's answer bot
// at d (the kernel rule's heldAnswer): d's pass when the window is open and
// bot casts a held card -- a plain choice of a held "cast" option, or a
// payment selection of a held card's payment action (d.PaymentActions). ok is false (bot
// stands) anywhere else, or when d offers no pass.
func heldAnswer(e *rules.Engine, d *decision.Decision, bot decision.Intent, s *boardScratch) (decision.Intent, bool) {
	if !s.holdOn(e, d) {
		return bot, false
	}
	var o decision.Option
	switch {
	case bot.Payment != nil:
		found := false
		for _, a := range d.PaymentActions {
			if a.ID == bot.Payment.ActionID {
				o, found = decision.Option{Kind: "cast", Obj: a.Cast.Object}, true
				break
			}
		}
		if !found {
			return bot, false
		}
	case len(bot.Choices) == 1 && bot.Choices[0] >= 0 && bot.Choices[0] < len(d.Options):
		o = d.Options[bot.Choices[0]]
	default:
		return bot, false
	}
	b := boardInto(e, d.Player, s)
	if !b.HeldUpkeepCast(e.G, o, s.hold.exempt) {
		return bot, false
	}
	for i, p := range d.Options {
		if p.Kind == "pass" {
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}, true
		}
	}
	return bot, false
}

// exemptNames is the names of ex, up to its first empty entry.
func exemptNames(ex *[MaxHoldUpkeepExempt]string) []string {
	for i, n := range ex {
		if n == "" {
			return ex[:i]
		}
	}
	return ex[:]
}
