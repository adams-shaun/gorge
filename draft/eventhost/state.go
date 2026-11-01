package eventhost

import (
	"fmt"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// stateFor builds one viewer's event state: seat -1 is the public
// spectator view (no You, no Join URLs, never another seat's private
// facts).
func (r *run) stateFor(seat int) protocol.EventState {
	st := protocol.EventState{
		Event: r.meta.ID, Set: r.meta.Set,
		Phase: string(r.ev.Phase()), Round: r.ev.Round(),
		BestOf: r.bestOf(),
		// The pick-timer mode as the meta stores it; a meta written
		// before the field existed reads back as "" (no timer).
		PickTimer: r.meta.PickTimer,
		Error:     firstNonEmpty(r.halted, r.blocked),
	}
	if seat >= 0 {
		s := seat
		st.You = &s
	}
	for s := 0; s < draft.Seats; s++ {
		ms := r.meta.Seats[s]
		es := protocol.EventSeat{
			Seat: s, Kind: ms.Kind, Name: ms.Name, Policy: ms.Policy, Drafter: ms.Drafter,
			Joined: ms.Kind == "bot" || ms.Name != "",
			Picks:  len(r.ev.Pool(s)),
			Ready:  r.ev.ReadyRev(s) != 0,
		}
		st.Seats = append(st.Seats, es)
	}
	results := r.ev.Results()
	for rd := 1; rd <= draft.Rounds; rd++ {
		for _, p := range event.Pairings(rd) {
			st.Series = append(st.Series, r.seriesFor(seat, results, rd, p[0], p[1]))
		}
	}
	// No standings rows until the first series has ended — a
	// pre-result table is empty, not a table of floor percentages.
	if len(results) > 0 {
		for _, s := range r.ev.Standings() {
			st.Standings = append(st.Standings, protocol.EventStanding{
				Seat: s.Seat, Points: s.Points, MW: s.MW, OMW: s.OMW, GW: s.GW, OGW: s.OGW,
			})
		}
	}
	return st
}

// bestOf is the event's series format: meta.BestOf 0 or 1 mean Bo1 (a meta
// written before the field existed, and Bo1 itself), 3 Bo3.
func (r *run) bestOf() int {
	if r.meta.BestOf == 3 {
		return 3
	}
	return 1
}

// seriesFor builds one pairing's row for a viewer. A Bo3 event's rows add
// the series' live games: score, per-game winners, the live game number
// and its play/draw chooser, and — between games — the seats still to
// submit their deck.
func (r *run) seriesFor(seat int, results []event.SeriesOutcome, rd, a, b int) protocol.EventSeries {
	if r.meta.BestOf == 3 {
		return r.seriesForBo3(seat, results, rd, a, b)
	}
	ser := protocol.EventSeries{Round: rd, A: a, B: b, BestOf: 1}
	for _, res := range results {
		if res.Round == rd && res.A == a && res.B == b {
			ser.State = "ended"
			ser.Score = res.Score
			if res.Winner >= 0 {
				w := res.Winner
				ser.Winner = &w
			}
			ser.Table = r.lastGameTable(rd, a, b)
			return ser
		}
	}
	if rd == r.ev.Round() && r.ev.Started(a, b) {
		// A result still waiting for its pairing-order log slot shows as
		// ended with its score and winner already; the log catches up in
		// flush's order, and standings stay log-derived.
		if o, ok := r.outcomes[[2]int{a, b}]; ok && o.m.State == protocol.MatchFinished {
			ser.State = "ended"
			ser.Table = o.m.Table
			if o.m.Winner != nil {
				w := a
				if *o.m.Winner == 1 {
					w = b
				}
				ser.Winner = &w
				if w == a {
					ser.Score = [2]int{1, 0}
				} else {
					ser.Score = [2]int{0, 1}
				}
			}
			return ser
		}
		ser.State = "live"
		for _, att := range r.ev.InFlight() {
			if att.A != a || att.B != b {
				continue
			}
			ser.Table, ser.Attempt = att.Table, att.Attempt
			joinLink(r, &ser, seat, a, b, att.Table)
			break
		}
		return ser
	}
	ser.State = "pending"
	return ser
}

// seriesForBo3 builds one Bo3 pairing's row for a viewer. An ended
// series shows its score, winner and per-game winners from the event's
// results; a live one its in-flight game, chooser and running score; a
// between one the next game, its chooser and the seats still to sideboard.
// A Bo1 series row never carries these fields.
func (r *run) seriesForBo3(seat int, results []event.SeriesOutcome, rd, a, b int) protocol.EventSeries {
	ser := protocol.EventSeries{Round: rd, A: a, B: b, BestOf: 3}
	for _, res := range results {
		if res.Round == rd && res.A == a && res.B == b {
			ser.State = "ended"
			ser.Score = res.Score
			ser.Games = res.Games
			if res.Winner >= 0 {
				w := res.Winner
				ser.Winner = &w
			}
			ser.Table = r.lastGameTable(rd, a, b)
			return ser
		}
	}
	if rd != r.ev.Round() || !r.ev.Started(a, b) {
		ser.State = "pending"
		return ser
	}
	v, ok := r.ev.SeriesState(a, b)
	if !ok {
		ser.State = "pending"
		return ser
	}
	ser.Score = v.Wins
	ser.Games = v.Games
	if v.InFlight != nil {
		ser.State = "live"
		ser.Table, ser.Attempt = v.InFlight.Table, v.InFlight.Attempt
		ser.Game = v.InFlight.Game
		if v.InFlight.Chooser >= 0 {
			c := v.InFlight.Chooser
			ser.Chooser = &c
		}
		joinLink(r, &ser, seat, a, b, v.InFlight.Table)
		return ser
	}
	if v.Between {
		ser.State = "between"
		ser.Table = r.lastGameTable(rd, a, b)
		ser.Game = v.NextGame
		if v.NextChooser >= 0 {
			c := v.NextChooser
			ser.Chooser = &c
		}
		for side, s := range []int{a, b} {
			if v.Locked[side] == 0 {
				ser.Waiting = append(ser.Waiting, s)
			}
		}
		return ser
	}
	// Started, next game not yet on the log (its GameStarted is between
	// here and the table): live like a Bo1 series awaiting its table.
	ser.State = "live"
	ser.Game = v.NextGame
	if v.NextGame > 1 && v.NextChooser >= 0 {
		c := v.NextChooser
		ser.Chooser = &c
	}
	return ser
}

// joinLink fills ser.Join for a viewer seated in the series: its own
// slot's game token (both humans of a series carry distinct ones)
// and the event id, so the game client knows which event the table
// belongs to. A spectator's row carries none.
func joinLink(r *run, ser *protocol.EventSeries, seat, a, b int, table string) {
	if (seat != a && seat != b) || r.tokens == nil {
		return
	}
	slot := 0
	if seat == b {
		slot = 1
	}
	tok := r.tokens[host.TableID(table)][slot]
	if tok == "" {
		return
	}
	ser.Join = fmt.Sprintf("/t/%s?seat=%d&token=%s&event=%s", table, slot, tok, r.meta.ID)
}

// lastGameTable is the table of the last GameStarted logged for an ended
// (round, a, b) series, so a spectator can still find its finished games.
func (r *run) lastGameTable(rd, a, b int) string {
	table := ""
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindGameStarted && en.Round == rd && en.A == a && en.B == b {
			table = en.Table
		}
	}
	return table
}

// packFor is a seat's draft_pack body: the pack at its queue head (or
// Waiting) and its pool so far.
func (r *run) packFor(seat int) protocol.DraftPack {
	out := protocol.DraftPack{
		Event: r.meta.ID, Seat: seat,
		Pool: r.draftCards(r.ev.Pool(seat)),
	}
	if r.ev.Phase() != event.PhaseDraft {
		out.Waiting = true
		return out
	}
	v, ok := r.ev.Next(seat)
	if !ok {
		out.Waiting = true
		return out
	}
	out.Round, out.PickNo = v.Round, v.PickNo
	out.Pack = r.draftCards(v.Pack)
	// The seat's armed pick deadline, when its armed key is still
	// the pack at its queue head.
	if t := r.timers[seat]; t.stop != nil && t.round == v.Round && t.pickNo == v.PickNo {
		out.TimeLimitMS = t.limit.Milliseconds()
		remaining := t.limit - r.h.clockNow().Sub(t.started)
		if remaining < 0 {
			remaining = 0
		}
		out.RemainingMS = remaining.Milliseconds()
	}
	return out
}

// firstNonEmpty returns the first non-empty string, or "".
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
