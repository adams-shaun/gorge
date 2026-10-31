package eventhost

import (
	"time"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// pickSeconds is XMage's per-pick limit by cards left in the pack: 15
// cards 75 s down to 1 card 7 s. Index 0 is a full 15-card pack.
var pickSeconds = [15]int{75, 70, 65, 60, 55, 50, 45, 40, 30, 25, 20, 15, 12, 10, 7}

// pickLimit is the time a seat has for one pick: XMage's table by the
// cards left (clamped to 1..15), ×1.5 in the relaxed mode, replaced
// wholesale by Options.PickTimeout when the operator set one. 0 for the
// "off" mode and for a meta written before the mode existed ("").
func (h *Host) pickLimit(mode string, cardsLeft int) time.Duration {
	if mode == "" || mode == "off" {
		return 0
	}
	if h.opts.PickTimeout > 0 {
		return h.opts.PickTimeout
	}
	if cardsLeft < 1 {
		cardsLeft = 1
	}
	if cardsLeft > 15 {
		cardsLeft = 15
	}
	d := time.Duration(pickSeconds[15-cardsLeft]) * time.Second
	if mode == "relaxed" {
		d = d * 3 / 2
	}
	return d
}

// pickTimer is one human seat's armed pick deadline, guarded by run.mu.
// round/pickNo name the pick it was armed for: a fire for a pick the seat
// no longer owes is dropped.
type pickTimer struct {
	round   int
	pickNo  int
	started time.Time
	limit   time.Duration
	stop    func() bool
}

// optsNow is the clock read of an Options: the injected pick-timer clock,
// else E1-8's Options.Now, else the real clock.
func optsNow(o Options) time.Time {
	if o.Clock != nil {
		return o.Clock.Now()
	}
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

// clockNow reads the host's clock.
func (h *Host) clockNow() time.Time { return optsNow(h.opts) }

// afterPickTimer arms one pick deadline through the injected clock, or
// the real one, and returns its stop.
func (h *Host) afterPickTimer(d time.Duration, f func()) (stop func() bool) {
	if h.opts.Clock != nil {
		return h.opts.Clock.AfterFunc(d, f)
	}
	return time.AfterFunc(d, f).Stop
}

// armPickTimers re-evaluates every seat's pick timer after the log moved.
// In DRAFT, with a timer mode on, every human seat owing a pick gets one
// timer for its current (round, pickNo) — stopped and re-armed whenever
// that pick changes — and a seat with nothing to pick has its timer
// stopped. Outside DRAFT, with the mode off or a halted run, every timer
// is stopped. It is called at the end of advance, under run.mu.
func (r *run) armPickTimers() {
	mode := r.meta.PickTimer
	if r.ev.Phase() != event.PhaseDraft || mode == "" || mode == "off" || r.halted != "" {
		r.stopPickTimers()
		return
	}
	now := r.h.clockNow()
	for s := 0; s < draft.Seats; s++ {
		if r.meta.Seats[s].Kind != "human" {
			r.stopPickTimer(s)
			continue
		}
		v, ok := r.ev.Next(s)
		if !ok {
			r.stopPickTimer(s)
			continue
		}
		if t := r.timers[s]; t.stop != nil && t.round == v.Round && t.pickNo == v.PickNo {
			continue
		}
		r.stopPickTimer(s)
		limit := r.h.pickLimit(mode, len(v.Pack))
		if limit <= 0 {
			continue
		}
		seat, round, pickNo := s, v.Round, v.PickNo
		r.timers[s] = pickTimer{
			round:   round,
			pickNo:  pickNo,
			started: now,
			limit:   limit,
			stop: r.h.afterPickTimer(limit, func() {
				r.h.pickExpired(r.meta.ID, seat, round, pickNo)
			}),
		}
	}
}

// stopPickTimer stops one seat's armed timer, if any, and clears its
// entry. The caller holds run.mu.
func (r *run) stopPickTimer(seat int) {
	if t := r.timers[seat]; t.stop != nil {
		t.stop()
	}
	r.timers[seat] = pickTimer{}
}

// stopPickTimers stops every seat's armed timer. The caller holds run.mu.
func (r *run) stopPickTimers() {
	for s := range r.timers {
		r.stopPickTimer(s)
	}
}

// pickExpired is one pick timer's callback. Like OnMatchEnd it takes
// Host.mu alone to check the host is still open and count the work on its
// wait group, never while a run mutex is held; the decision itself moves
// under run.mu.
func (h *Host) pickExpired(id string, seat, round, pickNo int) {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	r, ok := h.events[id]
	if ok {
		h.wg.Add(1)
	}
	h.mu.Unlock()
	if !ok {
		return
	}
	defer h.wg.Done()
	r.mu.Lock()
	r.pickExpired(seat, round, pickNo)
	r.mu.Unlock()
}

// pickExpired takes a seat's overdue pick with its own drafter and logs
// it as the host's action (Auto: true). A fire for a pick the seat no
// longer owes — picked by hand meanwhile, or the draft moved on — is
// dropped. It never refreshes the seat's idle clock: its human still has
// not acted. The caller holds run.mu.
func (r *run) pickExpired(seat, round, pickNo int) {
	if r.halted != "" || r.ev.Phase() != event.PhaseDraft {
		return
	}
	if r.meta.Seats[seat].Kind != "human" || r.drafters[seat] == nil {
		return
	}
	v, ok := r.ev.Next(seat)
	if !ok || v.Round != round || v.PickNo != pickNo {
		return
	}
	id := r.drafters[seat].Pick(v, pickRNG(r.meta.Seed, seat, round, pickNo))
	if err := r.apply(event.Entry{
		Kind: event.KindPicked, Round: round, PickNo: pickNo, Seat: seat, ID: id, Auto: true,
	}); err != nil {
		return
	}
	r.advance()
	r.broadcast()
}
