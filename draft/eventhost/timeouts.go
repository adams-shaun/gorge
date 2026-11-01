package eventhost

import (
	"fmt"
	"sort"
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/server/host"
)

// sweepEvery is the cadence of the abandonment sweep: every tick the host
// walks its runs and acts on any whose lobby or idle window has closed.
const sweepEvery = 30 * time.Second

// maxIdleSteps bounds one sweep's auto-action loop. A healthy path needs
// about as many steps as the event has remaining decisions; the cap only
// exists so a pathology cannot loop a sweep forever.
const maxIdleSteps = 256

// sweepLoop is the host's abandonment goroutine.
func (h *Host) sweepLoop() {
	defer h.wg.Done()
	t := time.NewTicker(sweepEvery)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			h.checkTimeouts(h.clockNow())
		case <-h.stop:
			return
		}
	}
}

// checkTimeouts runs one sweep pass over every registered run. It holds
// Host.mu only to snapshot the runs — never while a run's mutex is held.
func (h *Host) checkTimeouts(now time.Time) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	ids := make([]string, 0, len(h.events))
	for id := range h.events {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return idNum(ids[i]) < idNum(ids[j]) })
	h.mu.Unlock()
	for _, id := range ids {
		h.mu.Lock()
		r := h.events[id]
		h.mu.Unlock()
		if r == nil {
			continue
		}
		r.mu.Lock()
		r.expire(now)
		r.mu.Unlock()
	}
}

// expire is one sweep pass for one run. It closes an abandoned lobby —
// cancelled outright when no human ever joined, filled with bot seats when
// at least one did — and acts for human seats
// whose idle window closed with a decision owed. The sweep clock only ever
// APPENDS entries (LobbyTimeout / IdleTimeout markers, then the ordinary
// entries the actions take); replay never reads the clock. It is called
// under run.mu.
func (r *run) expire(now time.Time) {
	if r.halted != "" {
		return
	}
	if r.ev.Phase() == event.PhaseLobby {
		if r.lobbyAbandoned(now) {
			if r.joinedHumans() == 0 {
				if err := r.apply(event.Entry{Kind: event.KindLobbyTimeout}); err != nil {
					return
				}
			} else {
				if !r.fillLobbyWithBots() {
					return
				}
				r.advance()
			}
			r.broadcast()
		}
		return
	}
	acted := false
	for step := 0; step < maxIdleSteps; step++ {
		owed := r.idleOwed(now)
		if len(owed) == 0 {
			break
		}
		for _, s := range owed {
			_ = r.apply(event.Entry{Kind: event.KindIdleTimeout, Seat: s})
		}
		if !r.idleActions(now) {
			break
		}
		acted = true
	}
	if acted {
		r.broadcast()
	}
}

// lobbyAbandoned reports whether the lobby window closed with a human seat
// still unjoined.
func (r *run) lobbyAbandoned(now time.Time) bool {
	d := r.h.opts.LobbyTimeout
	if d <= 0 {
		return false
	}
	unjoined := false
	for _, ms := range r.meta.Seats {
		if ms.Kind == "human" && ms.Name == "" {
			unjoined = true
			break
		}
	}
	return unjoined && now.Sub(r.createdAt) > d
}

// joinedHumans counts the human seats that have joined the lobby. The
// caller holds run.mu.
func (r *run) joinedHumans() int {
	n := 0
	for _, ms := range r.meta.Seats {
		if ms.Kind == "human" && ms.Name != "" {
			n++
		}
	}
	return n
}

// fillLobbyWithBots converts every unjoined human seat of an abandoned
// lobby into a bot seat: the default drafter
// persona and the host's default game policy, named like a created bot.
// The seat's invite token dies with its humanity — the stored sha256 is
// cleared, so a stale join link can no longer act for the seat. The
// converted seats are logged as bots by the EventCreated the next advance
// applies. It saves the meta first and reports whether the event may go on;
// a persistence failure halts the run read-only instead. The caller holds
// run.mu.
func (r *run) fillLobbyWithBots() bool {
	policy, perr := r.h.botPolicy(r.h.opts.DefaultPolicy)
	if perr != nil {
		for _, cand := range r.h.opts.Policies {
			if p2, e2 := r.h.botPolicy(cand); e2 == nil {
				policy = p2
				break
			}
		}
	}
	if policy == "" {
		if r.halted == "" {
			r.halted = fmt.Sprintf("event %s: lobby fill found no bot policy", r.meta.ID)
		}
		return false
	}
	changed := false
	for s := range r.meta.Seats {
		ms := r.meta.Seats[s]
		if ms.Kind != "human" || ms.Name != "" {
			continue
		}
		ms.Kind = "bot"
		ms.Policy = policy
		ms.Drafter = "rating-colour"
		ms.Name = fmt.Sprintf("Bot %d (%s)", s+1, policy)
		ms.TokenSHA256 = ""
		r.meta.Seats[s] = ms
		changed = true
	}
	if !changed {
		return true
	}
	if err := r.h.opts.Store.saveMeta(r.meta); err != nil {
		if r.halted == "" {
			r.halted = fmt.Sprintf("event %s: persistence failed: %v", r.meta.ID, err)
		}
		return false
	}
	return true
}

// idleOwed lists the human seats whose idle window closed while a decision
// was owed, in seat order. IdleTimeout disabled (or non-positive) means no
// seat is ever owed.
func (r *run) idleOwed(now time.Time) []int {
	d := r.h.opts.IdleTimeout
	if d <= 0 {
		return nil
	}
	var out []int
	for s := 0; s < draft.Seats; s++ {
		if r.meta.Seats[s].Kind != "human" || now.Sub(r.lastAct[s]) <= d {
			continue
		}
		if r.owesDecision(s) {
			out = append(out, s)
		}
	}
	return out
}

// owesDecision reports whether the seat currently owes an action: a pack
// to pick from (DRAFT), an unsaved or unreadied deck (BUILD), or a series
// of its round that has not started or has a game in flight (ROUNDS).
func (r *run) owesDecision(seat int) bool {
	switch r.ev.Phase() {
	case event.PhaseDraft:
		_, ok := r.ev.Next(seat)
		return ok
	case event.PhaseBuild:
		return r.ev.SavedRev(seat) == 0 || r.ev.ReadyRev(seat) == 0
	case event.RoundPhase(1), event.RoundPhase(2), event.RoundPhase(3):
		for _, sp := range r.ev.PendingSeries() {
			if sp.A != seat && sp.B != seat {
				continue
			}
			if !r.ev.Started(sp.A, sp.B) || r.inFlight(sp.A, sp.B) != nil {
				return true
			}
			// Between games, a side still to submit its deck owes
			// one sideboarding submission.
			if v, ok := r.ev.SeriesState(sp.A, sp.B); ok && v.Between {
				side := 0
				if seat == sp.B {
					side = 1
				}
				if v.Locked[side] == 0 {
					return true
				}
			}
		}
		return false
	}
	return false
}

// inFlight is the series' in-flight game attempt, or nil.
func (r *run) inFlight(a, b int) *event.GameAttempt {
	for i, att := range r.ev.InFlight() {
		if att.A == a && att.B == b {
			return &r.ev.InFlight()[i]
		}
	}
	return nil
}

// idleActions takes every owed decision for every idle seat: draft
// auto-picks through the seat's own drafter, the builder's suggestion and
// a ready in BUILD, and a concession of the seat's remaining series in
// ROUNDS. An auto-action does not refresh lastAct: the seat stays idle
// until its human actually acts. It reports whether anything was applied.
// The caller holds run.mu.
func (r *run) idleActions(now time.Time) bool {
	acted := false
	for step := 0; step < maxIdleSteps; step++ {
		progressed := false
		switch r.ev.Phase() {
		case event.PhaseDraft:
			for _, s := range r.idleOwed(now) {
				if r.idlePick(s) {
					progressed, acted = true, true
				}
			}
		case event.PhaseBuild:
			for _, s := range r.idleOwed(now) {
				if r.idleBuild(s) {
					progressed, acted = true, true
				}
			}
		case event.RoundPhase(1), event.RoundPhase(2), event.RoundPhase(3):
			// An idle human in its series' between window keeps the
			// deck it played last game (one auto-keep per window) before
			// the concession path can end the series.
			if r.idleSideboard(now) {
				progressed, acted = true, true
			}
			if r.concedeIdle(now) {
				progressed, acted = true, true
			}
		default:
			return acted
		}
		if !progressed {
			return acted
		}
		r.advance()
	}
	return acted
}

// idlePick takes one pick for an idle seat with its own drafter. The
// Auto bit marks the entry as the host's action.
func (r *run) idlePick(seat int) bool {
	v, ok := r.ev.Next(seat)
	if !ok || r.drafters[seat] == nil {
		return false
	}
	id := r.drafters[seat].Pick(v, pickRNG(r.meta.Seed, seat, v.Round, v.PickNo))
	if err := r.apply(event.Entry{
		Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: seat, ID: id, Auto: true,
	}); err != nil {
		if r.halted == "" {
			r.halted = fmt.Sprintf("event %s: idle seat %d pick failed: %v", r.meta.ID, seat, err)
		}
		return false
	}
	return true
}

// idleBuild saves the builder's suggestion for an idle seat that has not
// saved one yet, and readies the seat whenever a deck is saved but not
// readied — a human who saved a deck and then vanished must not hold the
// event in BUILD forever. A suggestion that will not validate halts the
// run, exactly as the bot path in advanceBuild does.
func (r *run) idleBuild(seat int) bool {
	acted := false
	if r.ev.SavedRev(seat) == 0 {
		if r.builders[seat] == nil {
			return false
		}
		if _, err := r.saveDeck(seat, r.builders[seat].Build(r.meta.Set, r.ev.Pool(seat))); err != nil {
			if r.halted == "" {
				r.halted = fmt.Sprintf("event %s: idle seat %d could not save its deck: %v", r.meta.ID, seat, err)
			}
			return false
		}
		acted = true
	}
	if r.ev.ReadyRev(seat) == 0 {
		if err := r.apply(event.Entry{Kind: event.KindReady, Seat: seat, Rev: r.ev.SavedRev(seat)}); err != nil {
			return acted
		}
		acted = true
	}
	return acted
}

// concedeIdle concedes every pending series that contains an idle human
// seat, in pairing order. A series whose result is already buffered keeps
// its own result: flush logs it in pairing order. It reports whether
// anything was applied.
func (r *run) concedeIdle(now time.Time) bool {
	idle := map[int]bool{}
	any := false
	for s := 0; s < draft.Seats; s++ {
		if r.meta.Seats[s].Kind == "human" &&
			r.h.opts.IdleTimeout > 0 && now.Sub(r.lastAct[s]) > r.h.opts.IdleTimeout {
			idle[s], any = true, true
		}
	}
	if !any {
		return false
	}
	acted := false
	for _, sp := range r.ev.PendingSeries() {
		if !idle[sp.A] && !idle[sp.B] {
			continue
		}
		if _, buffered := r.outcomes[[2]int{sp.A, sp.B}]; buffered {
			continue
		}
		if err := r.concedeSeries(sp, idle); err != nil {
			return acted
		}
		acted = true
	}
	return acted
}

// concedeSeries closes one series by concession. Every game the
// series still owes is logged as a GameEnded with head "conceded": the
// in-flight game under its own attempt number, a never-started one under
// attempt 0. The absent human's side loses every remaining game; two
// absent humans draw every remaining game. A Bo3 series' SeriesEnded
// score is the wins its played games produced, so a forfeited
// series keeps the games its humans actually finished. A game attempt
// still in flight has its table deregistered first-class, so a late
// result from the stalled match is dropped by OnMatchEnd's table lookup.
func (r *run) concedeSeries(sp event.SeriesSpec, idle map[int]bool) error {
	if !r.ev.Started(sp.A, sp.B) {
		if err := r.apply(event.Entry{
			Kind: event.KindSeriesStarted, Round: sp.Round, A: sp.A, B: sp.B, BestOf: r.bestOf(),
		}); err != nil {
			return err
		}
	}
	var winner *int
	switch {
	case idle[sp.A] && idle[sp.B]:
	case idle[sp.A]:
		w := sp.B
		winner = &w
	default:
		w := sp.A
		winner = &w
	}
	// One GameEnded per pass, bounded by the series length: an in-flight
	// attempt is logged first, then the between window's never-started
	// remainder.
	for i := 0; i < r.bestOf(); i++ {
		v, ok := r.ev.SeriesState(sp.A, sp.B)
		if !ok || !v.Started || v.Ended || v.NextGame == 0 {
			break
		}
		if att := v.InFlight; att != nil {
			if err := r.apply(event.Entry{
				Kind: event.KindGameEnded, Round: sp.Round, A: sp.A, B: sp.B,
				Game: att.Game, Attempt: att.Attempt, Winner: winner, Head: "conceded",
			}); err != nil {
				return err
			}
			id := host.TableID(att.Table)
			r.h.mu.Lock()
			delete(r.h.tables, id)
			r.h.mu.Unlock()
			delete(r.tokens, id)
			continue
		}
		if err := r.apply(event.Entry{
			Kind: event.KindGameEnded, Round: sp.Round, A: sp.A, B: sp.B,
			Game: v.NextGame, Winner: winner, Head: "conceded",
		}); err != nil {
			return err
		}
	}
	v, ok := r.ev.SeriesState(sp.A, sp.B)
	if !ok {
		return nil
	}
	if err := r.apply(event.Entry{
		Kind: event.KindSeriesEnded, Round: sp.Round, A: sp.A, B: sp.B, Score: v.Wins,
	}); err != nil {
		return err
	}
	return nil
}

// idleSideboard locks the deck an idle human seat played last game for its
// series' next game (one auto-keep per between window, the host's
// action, Auto on the entry). When the next game then goes live and the
// seat still owes its game decisions, the sweep's concession path ends
// the series. It reports whether anything was applied. The caller holds
// run.mu.
func (r *run) idleSideboard(now time.Time) bool {
	acted := false
	for s := 0; s < draft.Seats; s++ {
		if r.meta.Seats[s].Kind != "human" || now.Sub(r.lastAct[s]) <= r.h.opts.IdleTimeout {
			continue
		}
		a, b, ok := r.ev.SeriesOf(s)
		if !ok {
			continue
		}
		v, ok := r.ev.SeriesState(a, b)
		if !ok || !v.Between || v.NextGame == 0 {
			continue
		}
		side := 0
		if s == b {
			side = 1
		}
		if v.Locked[side] != 0 {
			continue
		}
		if err := r.apply(event.Entry{
			Kind: event.KindSideboarded, Round: v.Round, A: a, B: b,
			Game: v.NextGame, Seat: s, Rev: v.PlayRevs[side], Auto: true,
		}); err != nil {
			if r.halted == "" {
				r.halted = fmt.Sprintf("event %s: idle seat %d sideboard failed: %v", r.meta.ID, s, err)
			}
			return acted
		}
		acted = true
	}
	return acted
}
