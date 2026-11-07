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
	// seat is the searching seat, whose decisions the hold always reads;
	// opp extends it to the other seats' own windows, under OpponentNodes
	// with OpponentMirror only (the kernel rule's oppHold): an unmirrored
	// opponent, or one whose decisions are not in the tree, is never held.
	seat state.PlayerID
	opp  bool
	// drops counts held casts dropped from a candidate list; Search reads
	// it after the root's enumeration (Stats.HeldUpkeep).
	drops int
}

// holdOn reports whether s arms the hold and decision d of e sits in its
// window: a priority decision of the active player in its untap, upkeep or
// draw step with an empty stack, the searching seat's own or, with
// upkeepHold.opp, another seat's. It is the cheap pre-check over the
// engine's own facts; the board predicate decides each option.
func (s *boardScratch) holdOn(e *rules.Engine, d *decision.Decision) bool {
	if s == nil || s.hold == nil || e == nil || d == nil || d.Kind != decision.KPriority {
		return false
	}
	if d.Player != s.hold.seat && !s.hold.opp {
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

// holdWalk is the hold at an in-walk decision pd (the kernel rule's
// holdWalk), after its enumeration made no point of it: a held bot answer
// in is played as pass (heldAnswer). drops is the hold's drop count before
// pd's enumeration; a decision the hold changed -- a candidate dropped or
// the answer replaced -- counts in Stats.HeldUpkeepWalk (the searching
// seat's) or HeldUpkeepOpp. It returns the answer to play; unarmed, in.
func (e *engineEnv) holdWalk(pd *decision.Decision, in decision.Intent, drops int) decision.Intent {
	s := e.cfg.enumBoard
	if s == nil || s.hold == nil {
		return in
	}
	if p, ok := heldAnswer(e.e, pd, in, s); ok {
		in = p
		s.hold.drops++
	}
	e.countHold(pd, drops)
	return in
}

// countHold counts decision pd in Stats.HeldUpkeepWalk or HeldUpkeepOpp
// when the hold's drop count moved past drops while answering it.
func (e *engineEnv) countHold(pd *decision.Decision, drops int) {
	s := e.cfg.enumBoard
	if s == nil || s.hold == nil || s.hold.drops == drops {
		return
	}
	if pd.Player == e.cfg.actor {
		e.cfg.stats.HeldUpkeepWalk++
	} else {
		e.cfg.stats.HeldUpkeepOpp++
	}
}

// holdDrops is the hold's drop count so far (0 unarmed).
func (s *boardScratch) holdDrops() int {
	if s == nil || s.hold == nil {
		return 0
	}
	return s.hold.drops
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
