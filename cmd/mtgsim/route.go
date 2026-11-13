package main

import (
	"context"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// seatRoute answers each decision the way the host answers a bot seat
// (server/host/match.go projectNext and parkSeat, server/host/botenv.go), so
// a game here plays the bots a hosted table plays:
//
//   - a bots.EnvSeat that wants the decision gets a bots.Env whose search
//     engine is the honest root searchseat.HonestRoot deals from the seat's
//     own observation feed, at bots.RootSeed(seed^(seat+1), d.Seq), never
//     the live engine. Without this branch az-redeal answered Decide, which
//     plays its wrapped heuristic bot, and never searched;
//   - a heuristic.BoardSeat answers DecideBoard from one reused Board, with
//     no View projected;
//   - any other seat answers Decide from a projected View.
//
// Every EnvSeat owns a feed that observes every decision of every player
// and records the seat's accepted answers, as the host's matchFeeds does. A
// game with no EnvSeat builds no feed and pays no capture.
type seatRoute struct {
	seed  uint64
	setup searchprobe.PublicGame
	// feeds is parallel to the seats: the feed of each EnvSeat, nil for
	// every other seat (and nil as a whole when there is no EnvSeat).
	// stopped marks a feed retired after a failed capture or record: its
	// history would miss an answer, so its seat plays its fallback.
	feeds   []*searchseat.Feed
	stopped []bool
	board   botpolicy.Board
	// rec is the actor's feed and the owned copy of the decision its
	// answer will be recorded against after the Submit (prepareRecord).
	rec    *searchseat.Feed
	recDec *decision.Decision
}

func newSeatRoute(cfg rules.Config, seats []engineseat.Seat) *seatRoute {
	r := &seatRoute{seed: cfg.Seed, board: botpolicy.NewBoard(len(seats))}
	for i, s := range seats {
		if _, ok := s.(bots.EnvSeat); !ok {
			continue
		}
		if r.feeds == nil {
			r.feeds = make([]*searchseat.Feed, len(seats))
			r.stopped = make([]bool, len(seats))
			r.setup = searchprobe.PublicGame{Names: cfg.Names, Decks: cfg.Decks, Tokens: cfg.Tokens, StartingLife: cfg.StartingLife}
		}
		r.feeds[i] = searchseat.NewFeed(state.PlayerID(i))
	}
	return r
}

// observe captures the current decision on every live feed (the host's
// matchFeeds.observe). A capture failure stops that one feed.
func (r *seatRoute) observe(e *rules.Engine) {
	for i, f := range r.feeds {
		if f == nil || r.stopped[i] || !f.Live() {
			continue
		}
		if _, ok := f.Observe(e); !ok {
			r.stopped[i] = true
		}
	}
}

// decide answers d, pending on e, with s, the seat d belongs to.
func (r *seatRoute) decide(ctx context.Context, e *rules.Engine, s engineseat.Seat, d *decision.Decision) (decision.Intent, error) {
	if es, ok := s.(bots.EnvSeat); ok && es.WantsEnv(d) {
		return r.decideEnv(ctx, e, es, d)
	}
	if bs, ok := s.(heuristic.BoardSeat); ok {
		return bs.DecideBoard(ctx, botpolicy.BoardFromGameInto(e.G, e, d.Player, &r.board), *d)
	}
	v := view.Project(e.G, e, d.Player, d)
	v.Round = view.RoundOf(e.G, e.L.Events)
	return s.Decide(ctx, v, *d)
}

// decideEnv is the host's envData plus parkSeat's DecideEnv call.
func (r *seatRoute) decideEnv(ctx context.Context, e *rules.Engine, es bots.EnvSeat, d *decision.Decision) (decision.Intent, error) {
	setup := searchseat.SeatSetup(r.setup, es, d.Player)
	var fd *searchseat.Feed
	if i := int(d.Player); r.feeds != nil && !r.stopped[i] {
		fd = r.feeds[i]
	}
	root, reason := searchseat.HonestRoot(setup, fd, e, bots.RootSeed(r.seed^(uint64(d.Player)+1), d.Seq))
	v := view.Project(e.G, e, d.Player, d)
	v.Round = view.RoundOf(e.G, e.L.Events)
	b := botpolicy.BoardFromGameInto(e.G, e, d.Player, &r.board).Detached()
	return es.DecideEnv(ctx, bots.Env{
		View:        v,
		Board:       b,
		Search:      searchseat.Env{Setup: setup, Engine: root, Board: b, Feed: fd},
		RootRefused: reason,
	}, *d)
}

// prepareRecord runs before the Submit of in, the answer to d: when d's
// seat has a live feed it keeps an owned copy of d (a Submit retires the
// decision's arena generation) to record the accepted answer against. A
// payment-plan answer names one of the decision's PaymentActions, which a
// seat that opts out of plans left unbuilt; they are built first (a pure
// derived read), as the host does.
func (r *seatRoute) prepareRecord(e *rules.Engine, d *decision.Decision, in decision.Intent) {
	r.rec, r.recDec = nil, nil
	i := int(d.Player)
	if r.feeds == nil || i >= len(r.feeds) || r.feeds[i] == nil || r.stopped[i] || !r.feeds[i].Live() {
		return
	}
	if in.Payment != nil || in.Announce != nil {
		e.EnsurePaymentActions()
	}
	r.rec, r.recDec = r.feeds[i], d.Clone()
}

// record files the accepted answer on the actor's feed after its Submit
// (the host's matchFeeds.record); a translation error stops the feed.
func (r *seatRoute) record(in decision.Intent) {
	if r.rec == nil {
		return
	}
	if err := r.rec.RecordAnswer(r.recDec, in); err != nil {
		r.stopped[r.recDec.Player] = true
	}
	r.rec, r.recDec = nil, nil
}
