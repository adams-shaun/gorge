package sbsearch

import (
	"math"

	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The seat's decision notes (bots/explain). They are built only when a driver
// installed a sink (SetExplain); with none, a decision builds nothing.
//
// Unit conversion: a root's mean is the searching seat's rollout value in
// [0,1] (1 a win, 0 a loss, 0.5 a draw, else the leaf), so a row's Q is
// explain.FromUnit of it. Every live root is rolled out once in every valid
// world, so a row's Visits is the valid world count, and 0 for a root the
// root decision refused. The search has no prior (Row.Prior 0) and no root
// value of its own: Search.RootValue is the played root's mean. Root 0 is
// sb-tactical's own answer, the row marked Bot.

var _ explain.Emitter = (*Seat)(nil)

// SetExplain installs the per-seat note sink (explain.Emitter); nil is off.
func (s *Seat) SetExplain(fn func(explain.Note)) { s.note = fn }

// searched is one decision's search as its note needs it.
type searched struct {
	roots           []root
	means           []float64 // nil when no world was valid
	valid, rollouts int
	choice          int
	refused         string
}

// noteDecision emits the note of one decision DecideSearch or DecideWorlds
// answered with in: sb-tactical's unsearched answer when nothing was
// searched (a kind or a decision with no real choice), sb-tactical's pick
// with the refusal when the search had no valid world, else the search's
// table.
func (s *Seat) noteDecision(d decision.Decision, in decision.Intent) {
	tr := s.last
	s.last = nil
	n := explain.Note{Seat: int(d.Player), Source: explain.Heuristic}
	if tr == nil {
		n.Bot = in.Choices
		s.note(n)
		return
	}
	switch {
	case tr.choice == 0:
		n.Bot = in.Choices
	case tr.roots[0].key == nil:
		n.Bot = tr.roots[0].in.Choices
	}
	if tr.valid == 0 || len(tr.means) != len(tr.roots) {
		n.Reason = tr.refused
		s.note(n)
		return
	}
	n.Source = explain.Policy
	visits := func(i int) int {
		if math.IsNaN(tr.means[i]) {
			return 0
		}
		return tr.valid
	}
	rows, dropped := explain.Rows(len(tr.roots), visits, tr.choice, 0, func(i int) explain.Row {
		r := explain.Row{Visits: visits(i), Q: explain.FromUnit(tr.means[i])}
		switch {
		case i == tr.choice:
			r.Choices = in.Choices
		case tr.roots[i].key == nil:
			r.Choices = tr.roots[i].in.Choices
		}
		return r
	})
	n.Search = &explain.Search{Engine: "sbsearch", Sims: tr.rollouts, RootValue: explain.FromUnit(tr.means[tr.choice]), Rows: rows, Dropped: dropped}
	s.note(n)
}
