package eventhost

import (
	"encoding/json"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
)

// broadcast pushes the event's frames to its subscribers. It runs under
// run.mu, after every successful mutation. Order per call: one event_state
// per subscriber, then (while drafting, or once just after the draft ends)
// one pod_progress per subscriber, then one draft_pack per seat whose pack
// key changed, to that seat's subscribers only — spectators never receive
// a draft_pack. A session that has gone missing or overflowed drops its
// subscription.
func (r *run) broadcast() {
	keep := r.subs[:0]
	for _, sub := range r.subs {
		s, ok := r.h.opts.Registry.Session(sub.session)
		if !ok {
			continue
		}
		if !r.push(s, protocol.TEventState, r.stateFor(sub.seat)) {
			continue
		}
		keep = append(keep, sub)
	}
	r.subs = keep

	if r.ev.Phase() == event.PhaseDraft || r.justDraft {
		for _, sub := range r.subs {
			s, ok := r.h.opts.Registry.Session(sub.session)
			if !ok {
				continue
			}
			r.push(s, protocol.TPodProgress, r.progress())
		}
	}
	r.justDraft = false

	for seat := 0; seat < draft.Seats; seat++ {
		key := r.packKey(seat)
		if key == r.lastPack[seat] {
			continue
		}
		r.lastPack[seat] = key
		for _, sub := range r.subs {
			if sub.seat != seat {
				continue
			}
			s, ok := r.h.opts.Registry.Session(sub.session)
			if !ok {
				continue
			}
			r.push(s, protocol.TDraftPack, r.packFor(seat))
		}
	}
	r.publishSeries()
}

// publishSeries pushes a series_update for each series whose public JSON
// changed, rounds then pairings in deterministic (pairing) order. It is
// called from broadcast under r.mu. lastSeries is looked up only by key,
// never ranged over, so map order never reaches a frame.
func (r *run) publishSeries() {
	results := r.ev.Results()
	for rd := 1; rd <= draft.Rounds; rd++ {
		for _, p := range event.Pairings(rd) {
			ser := r.seriesFor(-1, results, rd, p[0], p[1])
			b, err := json.Marshal(ser)
			if err != nil {
				continue // a marshal failure of our own struct cannot happen
			}
			key := [3]int{rd, p[0], p[1]}
			if r.lastSeries[key] == string(b) {
				continue
			}
			r.lastSeries[key] = string(b)
			for _, sub := range r.subs {
				s, ok := r.h.opts.Registry.Session(sub.session)
				if !ok {
					continue
				}
				r.push(s, protocol.TSeriesUpdate, protocol.SeriesUpdate{
					Event: r.meta.ID, Series: ser,
				})
			}
		}
	}
}

// push builds and publishes one frame; it reports whether the session
// took it.
func (r *run) push(s *host.Session, t protocol.FrameType, body any) bool {
	f, err := protocol.NewFrame(t, "", 0, 0, body)
	if err != nil {
		return false // a marshal failure of our own struct cannot happen
	}
	return r.h.opts.Registry.Publish(s, f)
}

// progress is the pod_progress body: public counts only.
func (r *run) progress() protocol.PodProgress {
	out := protocol.PodProgress{Event: r.meta.ID, Round: r.currentDraftRound()}
	for s := 0; s < draft.Seats; s++ {
		out.Picks = append(out.Picks, len(r.ev.Pool(s)))
		out.Queued = append(out.Queued, r.ev.Queued(s))
	}
	return out
}

// currentDraftRound is the round the pod is drafting: the round of any
// queued pack, else the highest round a pick names.
func (r *run) currentDraftRound() int {
	for s := 0; s < draft.Seats; s++ {
		if v, ok := r.ev.Next(s); ok {
			return v.Round
		}
	}
	round := 1
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPicked && en.Round > round {
			round = en.Round
		}
	}
	return round
}
