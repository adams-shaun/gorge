package searchseat

import (
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The teacher seat's decision notes (bots/explain). They run only when a
// driver installed a sink (SetExplain); with none, DecideSearch builds
// nothing.
//
// Unit conversion: searchprobe.TeacherResult.Values are per-candidate mean
// rollout values in [0,1] from the deciding seat's point of view, so a row's
// Q is explain.FromUnit of its value. Every candidate is rolled out once in
// every sampled world, so a row's Visits is Trace.Worlds. The teacher has no
// prior (Row.Prior 0) and no root value of its own: Search.RootValue is the
// played candidate's value.

var _ explain.Emitter = (*SearchBot)(nil)

// SetExplain installs the per-seat note sink (explain.Emitter); nil is off.
func (s *SearchBot) SetExplain(fn func(explain.Note)) { s.note = fn }

// noteChoose emits the note of one DecideSearch: the bot's answer when the
// teacher did not answer, the teacher's table when it scored the decision.
// tr is Choose's trace (nil when Choose was not called: a stopped feed or an
// ineligible kind), in the intent played.
//
//go:noinline
func (s *SearchBot) noteChoose(env Env, d decision.Decision, bot, in decision.Intent, tr *Trace, reason string) {
	n := explain.Note{Seat: int(d.Player), Source: explain.Heuristic, Reason: reason, Bot: bot.Choices}
	switch {
	case tr == nil:
	case len(tr.Candidates) < 2 && tr.Kind != "":
		n.Source = explain.Forced
	case !tr.Covered:
		n.Reason = tr.Fallback
	default:
		n.Source = explain.Policy
		if tr.Fallback != "" {
			// Scored, but the teacher's pick did not translate back
			// ("root match"): the bot's answer was played.
			n.Source, n.Reason = explain.Heuristic, tr.Fallback
		}
	}
	if tr != nil && tr.Covered && len(tr.Values) == len(tr.Candidates) {
		n.Search = teacherRows(env, &d, bot, in, tr)
	}
	s.note(n)
}

// teacherRows is a scored decision's Search table.
func teacherRows(env Env, d *decision.Decision, bot, in decision.Intent, tr *Trace) *explain.Search {
	chosen := tr.Index
	if tr.Fallback != "" {
		chosen = 0
	}
	rows, dropped := explain.Rows(len(tr.Candidates), func(int) int { return tr.Worlds }, chosen, 0, func(i int) explain.Row {
		r := explain.Row{Visits: tr.Worlds, Q: explain.FromUnit(tr.Values[i])}
		switch {
		case i == 0:
			r.Choices = bot.Choices
		case i == chosen:
			r.Choices = in.Choices
		case env.Feed != nil:
			if m, err := env.Feed.Collector().Match(d, tr.Candidates[i]); err == nil {
				r.Choices = m.Choices
			}
		}
		return r
	})
	return &explain.Search{Engine: "searchseat", Sims: tr.Rollouts, RootValue: explain.FromUnit(tr.Values[chosen]), Rows: rows, Dropped: dropped}
}
