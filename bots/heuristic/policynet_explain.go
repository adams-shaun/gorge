package heuristic

import (
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// PolicyNetBot's decision notes (bots/explain). They run only when a driver
// installed a sink (SetExplain); with none, Decide builds nothing.
//
// Unit conversion: the checkpoint's value head is a sigmoid, V(s) in [0,1]
// for the deciding seat, so Net.Value is explain.FromUnit of it. Net.Scores
// are the raw per-option scores (head plus residual) the answer was read
// from, in decision option order -- the BotChosen positions the recorder
// reports index the same list.

var _ explain.Emitter = (*PolicyNetBot)(nil)

// SetExplain installs the per-seat note sink (explain.Emitter); nil is off.
func (b *PolicyNetBot) SetExplain(fn func(explain.Note)) { b.note = fn }

// noteBot emits a note for a decision answered by the wrapped default bot
// without scoring.
//
//go:noinline
func (b *PolicyNetBot) noteBot(p state.PlayerID, in decision.Intent, reason string) {
	b.note(explain.Note{Seat: int(p), Source: explain.Heuristic, Reason: reason, Bot: in.Choices})
}

// noteScored emits a scored decision's note: Policy when the answer was read
// from the scores, Heuristic (the bot's answer stood) when the scored surface
// declined to answer.
//
//go:noinline
func (b *PolicyNetBot) noteScored(p state.PlayerID, st policynet.State, scores []float32, botIn decision.Intent, scored bool) {
	n := explain.Note{Seat: int(p), Source: explain.Policy, Bot: botIn.Choices, Net: &explain.Net{}}
	if !scored {
		n.Source, n.Reason = explain.Heuristic, "scores declined"
	}
	n.Net.Scores = make([]float64, len(scores))
	for i, x := range scores {
		n.Net.Scores[i] = explain.Round(float64(x))
	}
	if b.scorer != nil && b.scorer.HasValue() {
		n.Net.Value = explain.FromUnit(float64(b.scorer.Value(st)))
	}
	b.note(n)
}
