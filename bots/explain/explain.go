// Package explain is the per-intent decision note a match record carries
// beside its log (match-reviewer design §3, D6): who answered one decision,
// by which route, and -- for a search or a scoring net -- what it saw.
//
// Record.Explain[i] (server/matchrecord) describes Record.Log.Intents[i]:
// either the slice is empty (no explain was recorded) or it has one entry per
// intent, JSON null where nothing was noted. The note is never replay input
// and never hashed.
//
// A seat that searches or scores implements Emitter; a driver that records
// installs a per-seat func with SetExplain, and the seat calls it once per
// decision it answers. A nil func is off, and off costs a nil check: no Note
// is built, no row slice allocated, nothing copied. Plain heuristic seats
// have no emitter; the driver tags their intents Heuristic.
//
// Conventions every emitter follows:
//
//   - Choices are option indices of the decision, the same numbers as the
//     logged intent's Choices (decision.Intent.Choices).
//   - Every value (Row.Q, Search.RootValue, Net.Value) is from the DECIDING
//     seat's perspective, in [-1,1]: +1 a sure win, -1 a sure loss, 0 even.
//     The searches and nets in gorge keep values in [0,1] (azmcts's leaf,
//     searchprobe's teacher means, policynet's sigmoid value head), so an
//     emitter converts with FromUnit.
//   - Floats are rounded to three decimals (Round), which is what keeps a
//     2-seat game's gzipped record inside the design's 64 KB budget.
//
// The package is a leaf: it imports only the standard library, so any bot,
// driver or server package may depend on it.
package explain

import (
	"math"
	"sort"
)

// Source records who actually answered a decision (design §3, D6).
type Source string

const (
	// Policy: the net or search answered.
	Policy Source = "policy"
	// Heuristic: the seat's own bot answered (a plain bot seat, or a kind
	// the seat does not search or score). Reason may say why a search seat
	// played its bot at a kind it does search (a failed search, a refused
	// redeal, a deadline).
	Heuristic Source = "heuristic"
	// Forced: not searched, because the decision had fewer than two
	// candidates in the search's vocabulary. azmcts counts here a bot answer
	// the vocabulary cannot express (a land play or a mana tap at priority:
	// its SkipFewCandidates), so a forced priority note is not always a
	// pass-only window.
	Forced Source = "forced"
	// PriorSkip: the decision had more options than azmcts's
	// PriorRankMaxOptions, so the network did not rank its candidates. In
	// azmcts that skips only the RANKING, not the search: the decision is
	// still searched, over the enumeration's first candidates under a uniform
	// prior, so a PriorSkip note carries Search like a Policy one.
	PriorSkip Source = "prior_skip"
	// FallbackFeed: the driver routed around the search because the seat's
	// observation feed had stopped (driver).
	FallbackFeed Source = "fallback_feed"
	// FallbackRoot: the driver routed around the search because the honest
	// root was refused; Reason says why (driver).
	FallbackRoot Source = "fallback_root"
	// RefusalRetry: the engine refused the seat's answer and the seat's
	// AnswerRefused answer was accepted (driver).
	RefusalRetry Source = "refusal_retry"
	// RefusalFallback: the engine refused the seat's answer and a shared
	// bots.Fallbacks rung was accepted; Rung says which (driver).
	RefusalFallback Source = "refusal_fallback"
)

// Note is one intent's explanation.
type Note struct {
	Seat int `json:"seat"`
	// Policy is the registry name of the seat's policy, when the driver
	// knows it.
	Policy string `json:"policy,omitempty"`
	Source Source `json:"source"`
	// Reason is the FallbackRoot / Refusal* detail, or why a search seat
	// played its bot (Heuristic).
	Reason string `json:"reason,omitempty"`
	// Rung is RefusalFallback's 1-based bots.Fallbacks index.
	Rung int `json:"rung,omitempty"`
	// Bot is the wrapped heuristic's answer when the seat knows it: for a
	// search, candidate 0's Choices (its own bot's answer).
	Bot []int `json:"bot,omitempty"`
	// Search is the search the seat ran, when it ran one. A driver source
	// (FallbackFeed, FallbackRoot, Refusal*) keeps the seat's Search, so a
	// reviewer sees what the search wanted even though it was refused.
	Search *Search `json:"search,omitempty"`
	// Net is a scoring net's view of the decision.
	Net *Net `json:"net,omitempty"`
}

// Search is one search's root table and cost.
type Search struct {
	// Engine is "azmcts" or "searchseat".
	Engine string `json:"engine"`
	// Sims is the number of simulations that completed (azmcts) or the
	// rollouts run (searchseat).
	Sims int `json:"sims,omitempty"`
	// RootValue is the deciding seat's value of the position, in [-1,1]:
	// azmcts's root mean; searchseat has no root value of its own, so it is
	// the played candidate's mean.
	RootValue float64 `json:"root_value"`
	// Cheap marks a playout-cap cheap search (azmcts SeatConfig.FullProb).
	Cheap bool `json:"cheap,omitempty"`
	// MS is the decision's wall milliseconds, 0 when the driver installed no
	// clock. Never replay input.
	MS float64 `json:"ms,omitempty"`
	// Rows are the root candidates, most-visited first (ties on candidate
	// order): the top MaxRows, plus the chosen and bot rows when they fall
	// outside.
	Rows []Row `json:"rows"`
	// Dropped is the number of candidates not listed.
	Dropped int `json:"dropped,omitempty"`
}

// Row is one root candidate.
type Row struct {
	// Choices is the candidate intent's Choices.
	Choices []int `json:"choices"`
	// Visits is the candidate's visit count: azmcts's root-child visits;
	// searchseat's worlds, since every candidate is rolled out once per
	// sampled world.
	Visits int `json:"visits"`
	// Prior is the candidate's prior before root noise (azmcts); searchseat
	// has none and writes 0.
	Prior float64 `json:"prior"`
	// Q is the candidate's mean backed-up value, deciding seat's
	// perspective, in [-1,1]; 0 for an unvisited candidate.
	Q float64 `json:"q"`
	// Chosen marks the candidate the seat played.
	Chosen bool `json:"chosen,omitempty"`
	// Bot marks candidate 0, the heuristic's own answer. A root built
	// without the bot's answer (azmcts's tap reroot, Root.NoBot) marks none.
	Bot bool `json:"bot,omitempty"`
}

// Net is a scoring net's view of one decision.
type Net struct {
	// Value is the net's value of the position, deciding seat's
	// perspective, in [-1,1].
	Value float64 `json:"value"`
	// Scores are the net's per-option scores (logits, head plus residual),
	// one per decision option in option order; omitted when the net did not
	// score the options.
	Scores []float64 `json:"scores,omitempty"`
}

// Emitter is a seat that can explain its decisions. SetExplain installs the
// per-seat sink the seat calls once for each decision it answers; nil turns
// it off. A wrapper forwards SetExplain to the seat it wraps.
type Emitter interface{ SetExplain(func(Note)) }

// MaxRows is how many most-visited rows a Search lists before the chosen and
// bot rows are added.
const MaxRows = 16

// Round rounds v to three decimals (NaN and infinities read 0).
func Round(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*1000) / 1000
}

// FromUnit converts a [0,1] value (0 a loss, 1 a win, 0.5 even) to the
// note's [-1,1] scale, rounded.
func FromUnit(v float64) float64 { return Round(2*v - 1) }

// Rows builds a Search's rows over n candidates: the MaxRows most-visited
// (ties on the lower candidate index), plus chosen and bot when they are not
// among them, ordered most-visited first. visits(i) is candidate i's visit
// count and row(i) builds its row; Rows sets Chosen and Bot itself. bot < 0
// means no candidate is the bot's. dropped is the number of candidates not
// listed.
func Rows(n int, visits func(i int) int, chosen, bot int, row func(i int) Row) (rows []Row, dropped int) {
	if n <= 0 {
		return nil, 0
	}
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return visits(order[a]) > visits(order[b]) })
	keep := order
	if len(keep) > MaxRows {
		keep = append(make([]int, 0, MaxRows+2), order[:MaxRows]...)
		for _, extra := range [2]int{chosen, bot} {
			if extra < 0 || extra >= n || contains(keep, extra) {
				continue
			}
			keep = append(keep, extra)
		}
		// The extras were appended last; restore most-visited order over the
		// kept set.
		sort.SliceStable(keep, func(a, b int) bool {
			va, vb := visits(keep[a]), visits(keep[b])
			if va != vb {
				return va > vb
			}
			return keep[a] < keep[b]
		})
	}
	rows = make([]Row, len(keep))
	for j, i := range keep {
		r := row(i)
		r.Chosen, r.Bot = i == chosen, i == bot
		rows[j] = r
	}
	return rows, n - len(keep)
}

func contains(xs []int, x int) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
