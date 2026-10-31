package event

import (
	"sort"

	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// GameOutcome is one played game's result relative to the series' seats:
// AWon means seat A won, BWon means seat B won, Draw means neither.
type GameOutcome int8

// The game outcomes.
const (
	AWon GameOutcome = iota
	BWon
	Draw
)

// SeriesResult is one completed series' result, in the shape
// ComputeStandings takes. A must be the lower seat; Score counts A's and B's
// series wins and Games lists the played games in order.
type SeriesResult struct {
	Round int
	A, B  int
	Score [2]int
	Games []GameOutcome
}

// Standing is one seat's row of the standings table, MTR Appendix C style.
// MW, OMW, GW and OGW are the match-win, opponent match-win, game-win and
// opponent game-win percentages, each floored at 33% where the appendix
// says so.
type Standing struct {
	Seat   int     `json:"seat"`
	Points int     `json:"points"`
	MW     float64 `json:"mw"`
	OMW    float64 `json:"omw"`
	GW     float64 `json:"gw"`
	OGW    float64 `json:"ogw"`
}

// floorPercent is MTR Appendix C's 33% floor, applied to a player's own MW%
// and GW% (and, through the opponents' floored values, to OMW% and OGW%).
const floorPercent = 0.33

// ComputeStandings builds the standings table for the hosted 4-seat pod
// from completed series results; see ComputeStandingsN.
func ComputeStandings(results []SeriesResult) []Standing {
	return ComputeStandingsN(draft.Seats, results)
}

// ComputeStandingsN builds an n-row standings table from completed series
// results: match points 3/1/0 per MTR Appendix C, MW% = points/(3×series)
// and GW% = game points/(3×games) each floored at 0.33, OMW% and OGW% the
// means of the opponents' floored values, ordered by points, OMW%, GW%,
// OGW%, then seat index. It accepts any subset of a round robin's series,
// so mid-event tables compute the same way. Results naming seats outside
// 0..n-1 are ignored.
func ComputeStandingsN(n int, results []SeriesResult) []Standing {
	aggs := make([]seatAgg, n)
	for _, r := range results {
		lo, hi := r.A, r.B
		if lo > hi {
			lo, hi = hi, lo
		}
		if lo < 0 || hi >= n || lo == hi {
			continue
		}
		switch {
		case r.Score[0] > r.Score[1]:
			aggs[lo].points += 3
		case r.Score[0] < r.Score[1]:
			aggs[hi].points += 3
		default:
			aggs[lo].points++
			aggs[hi].points++
		}
		aggs[lo].series++
		aggs[hi].series++
		aggs[lo].opps = append(aggs[lo].opps, hi)
		aggs[hi].opps = append(aggs[hi].opps, lo)
		for _, g := range r.Games {
			aggs[lo].games++
			aggs[hi].games++
			switch g {
			case AWon:
				aggs[lo].gamePts += 3
			case BWon:
				aggs[hi].gamePts += 3
			case Draw:
				aggs[lo].gamePts++
				aggs[hi].gamePts++
			}
		}
	}
	out := make([]Standing, len(aggs))
	for i := range out {
		a := &aggs[i]
		var omw, ogw float64
		if len(a.opps) > 0 {
			var sMW, sGW float64
			for _, o := range a.opps {
				sMW += pct(aggs[o].points, aggs[o].series)
				sGW += pct(aggs[o].gamePts, aggs[o].games)
			}
			omw = sMW / float64(len(a.opps))
			ogw = sGW / float64(len(a.opps))
		}
		out[i] = Standing{
			Seat:   i,
			Points: a.points,
			MW:     pct(a.points, a.series),
			OMW:    omw,
			GW:     pct(a.gamePts, a.games),
			OGW:    ogw,
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Points != out[j].Points {
			return out[i].Points > out[j].Points
		}
		if out[i].OMW != out[j].OMW {
			return out[i].OMW > out[j].OMW
		}
		if out[i].GW != out[j].GW {
			return out[i].GW > out[j].GW
		}
		if out[i].OGW != out[j].OGW {
			return out[i].OGW > out[j].OGW
		}
		return out[i].Seat < out[j].Seat
	})
	return out
}

// seatAgg accumulates one seat's results across the standings input.
type seatAgg struct {
	points  int
	series  int
	gamePts int
	games   int
	opps    []int
}

// pct is a win percentage: points over 3×n, floored at 0.33; a seat with no
// played series has no defined percentage and scores 0.
func pct(points, n int) float64 {
	if n == 0 {
		return 0
	}
	v := float64(points) / (3 * float64(n))
	if v < floorPercent {
		return floorPercent
	}
	return v
}

// SeriesResults lists the event's finished series as the standings input
// shape, in the order they ended: the conversion the standings table and
// the Swiss pairing function both read.
func (e *Event) SeriesResults() []SeriesResult {
	rs := make([]SeriesResult, 0, len(e.results))
	for _, f := range e.results {
		games := make([]GameOutcome, len(f.games))
		for i, g := range f.games {
			switch {
			case g < 0:
				games[i] = Draw
			case int(g) == f.a:
				games[i] = AWon
			default:
				games[i] = BWon
			}
		}
		rs = append(rs, SeriesResult{Round: f.round, A: f.a, B: f.b, Score: f.score, Games: games})
	}
	return rs
}

// Standings computes the event's current table from its completed series.
func (e *Event) Standings() []Standing {
	return ComputeStandingsN(e.NumSeats(), e.SeriesResults())
}
