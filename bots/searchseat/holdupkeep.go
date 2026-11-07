package searchseat

import (
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// holdWindow reports whether Options.HoldUpkeep is on and decision d of e
// sits in its window: a priority decision of the active player in its untap,
// upkeep or draw step with an empty stack. It is the cheap pre-check over the
// engine's own facts; botpolicy.Board.HeldUpkeepCast decides each option.
func holdWindow(e *rules.Engine, d *decision.Decision, opts Options) bool {
	if !opts.HoldUpkeep || e == nil || d == nil || d.Kind != decision.KPriority {
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

// holdAnswer is the intent Choose plays in place of the bot's answer bot: d's
// pass when the hold's window is open and bot is a single held "cast"
// choice, with held 1; bot and 0 anywhere else, or when d offers no pass.
func holdAnswer(e *rules.Engine, d *decision.Decision, bot decision.Intent, opts Options) (decision.Intent, int) {
	if !holdWindow(e, d, opts) || bot.Payment != nil || len(bot.Choices) != 1 || bot.Choices[0] < 0 || bot.Choices[0] >= len(d.Options) {
		return bot, 0
	}
	b := botpolicy.BoardFromGame(e.G, e, d.Player)
	if !b.HeldUpkeepCast(e.G, d.Options[bot.Choices[0]], opts.HoldUpkeepExempt) {
		return bot, 0
	}
	for i, o := range d.Options {
		if o.Kind == "pass" {
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}, 1
		}
	}
	return bot, 0
}

// holdOptions is the cast arm's observed decision od without the held casts
// (od.Options[i] is d.Options[i]). od is returned unchanged when the hold is
// off, outside its window, or nothing is held.
func holdOptions(e *rules.Engine, d *decision.Decision, od *searchprobe.ObservedDecision, opts Options) *searchprobe.ObservedDecision {
	if od == nil || len(od.Options) != len(d.Options) || !holdWindow(e, d, opts) {
		return od
	}
	b := botpolicy.BoardFromGame(e.G, e, d.Player)
	var keep []searchprobe.ObservedOption
	for i, o := range d.Options {
		if b.HeldUpkeepCast(e.G, o, opts.HoldUpkeepExempt) {
			if keep == nil {
				keep = append(make([]searchprobe.ObservedOption, 0, len(od.Options)), od.Options[:i]...)
			}
			continue
		}
		if keep != nil {
			keep = append(keep, od.Options[i])
		}
	}
	if keep == nil {
		return od
	}
	out := *od
	out.Options = keep
	return &out
}
