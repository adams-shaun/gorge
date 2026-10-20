package azmcts

import (
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// The az seat's decision notes (bots/explain, match-reviewer design §3).
// Everything here runs only when a driver installed a sink (SetExplain): the
// seat's Decide paths test s.note for nil and call in, so with no sink the
// seat builds no note, no row and no copy.
//
// Unit conversion: azmcts keeps every value in [0,1] from the searching
// seat's perspective (Leaf.V: 1 a win, 0 a loss; TreeResult.Q and RootValue
// are means of it), so a note's Q, RootValue and Net.Value are
// explain.FromUnit of them. An unvisited candidate's Q (0 in the Result) is
// written 0, the note's neutral value, not FromUnit(0) = -1.

var _ explain.Emitter = (*Seat)(nil)

// SetExplain installs the per-seat note sink (explain.Emitter); nil is off.
func (s *Seat) SetExplain(fn func(explain.Note)) { s.note = fn }

// noteBot emits a note for a decision the seat answered with its own bot
// without running a search.
//
//go:noinline
func (s *Seat) noteBot(p state.PlayerID, src explain.Source, reason string, bot decision.Intent) {
	s.note(explain.Note{Seat: int(p), Source: src, Reason: reason, Bot: bot.Choices})
}

// skipWhy is the reason Search counted a skipped decision under, read back
// off its kind's KindSkipped (or ExtraSkipped) row: Search counts exactly
// one skip per skipped root.
func skipWhy(res *Result) (SkipReason, bool) {
	var row *[NumSkipReasons]int
	if k := kindIndex(res.Kind); k >= 0 {
		row = &res.Stats.KindSkipped[k]
	} else if x := extraIndex(res.Kind); x >= 0 {
		row = &res.Stats.ExtraSkipped[x]
	} else {
		return 0, false
	}
	for why, n := range row {
		if n > 0 {
			return SkipReason(why), true
		}
	}
	return 0, false
}

// searchNote is one DecideSearch outcome as the seat's note.
type searchNote struct {
	d          *decision.Decision
	bot        decision.Intent
	res        Result
	root       *rules.Engine
	refused    string
	cheap      bool
	noBot      bool // the root was built without the bot's answer (tap reroot)
	rankSkip   bool // the root's network ranking was skipped (PriorRankMaxOptions)
	ms         float64
	searchedOK bool // the search completed and its choice was played
}

// noteSearch emits the note of one DecideSearch that reached Search.
//
//go:noinline
func (s *Seat) noteSearch(sn searchNote) {
	res := &sn.res
	n := explain.Note{Seat: int(sn.d.Player), Source: explain.Heuristic, Bot: sn.bot.Choices}
	searched := res.Stats.Searched == 1
	switch {
	case res.Kind == "":
		// Not a searched kind.
	case !searched && len(res.Candidates) == 0:
		if why, ok := skipWhy(res); ok {
			if why == SkipFewCandidates {
				n.Source = explain.Forced
			} else {
				n.Reason = SkipReasonNames[why]
			}
		}
	case !searched:
		n.Reason = "no simulations"
	case res.Stats.DeadlineHits > 0:
		n.Reason = "deadline"
	case res.Stats.AllFailed == 1:
		n.Reason = "every simulation failed"
		if sn.refused != "" {
			n.Reason = "redeal refused: " + sn.refused
		}
	case sn.rankSkip:
		n.Source = explain.PriorSkip
	default:
		n.Source = explain.Policy
	}
	if len(res.Candidates) >= 2 {
		n.Search = s.searchRows(&sn)
		if s.net != nil && s.net.HasValue() && sn.root != nil {
			n.Net = &explain.Net{Value: explain.FromUnit(leafValue(s.net, sn.root, sn.d.Player, nil))}
		}
	}
	s.note(n)
}

// searchRows is the note's Search: the root table, the cost and the value.
func (s *Seat) searchRows(sn *searchNote) *explain.Search {
	res := &sn.res
	bot := 0
	if sn.noBot {
		bot = -1
	}
	chosen := bot
	if sn.searchedOK {
		chosen = res.Choice
	} else if sn.noBot {
		// The bot's answer was played but is no candidate.
		chosen = -1
	}
	visits := func(i int) int {
		if i < len(res.Visits) {
			return res.Visits[i]
		}
		return 0
	}
	rows, dropped := explain.Rows(len(res.Candidates), visits, chosen, bot, func(i int) explain.Row {
		r := explain.Row{Choices: res.Candidates[i].Choices, Visits: visits(i)}
		if i < len(res.Prior) {
			r.Prior = explain.Round(res.Prior[i])
		}
		if r.Visits > 0 && i < len(res.Q) {
			r.Q = explain.FromUnit(res.Q[i])
		}
		return r
	})
	out := &explain.Search{Engine: "azmcts", Sims: res.Stats.Completed, Cheap: sn.cheap, MS: explain.Round(sn.ms), Rows: rows, Dropped: dropped}
	if res.Stats.Completed > 0 {
		out.RootValue = explain.FromUnit(res.RootValue)
	}
	return out
}

// notePrior emits the PriorOnly seat's note: no simulation, the argmax of
// the network prior over the candidates.
//
//go:noinline
func (s *Seat) notePrior(p state.PlayerID, bot decision.Intent, res Result, best int) {
	n := explain.Note{Seat: int(p), Source: explain.Policy, Bot: bot.Choices}
	rows, dropped := explain.Rows(len(res.Candidates), func(int) int { return 0 }, best, 0, func(i int) explain.Row {
		return explain.Row{Choices: res.Candidates[i].Choices, Prior: explain.Round(res.Prior[i])}
	})
	n.Search = &explain.Search{Engine: "azmcts", Rows: rows, Dropped: dropped}
	s.note(n)
}
