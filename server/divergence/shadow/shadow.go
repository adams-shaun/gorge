// Package shadow computes the shadow answer B of the human-divergence
// capture (design 2026-10-10-human-divergence-design.md §3, §5.4): the
// intent a fresh, deterministically seeded instance of a hosted bot policy
// would give at an engine's pending decision, built from exactly the inputs
// the host's dispatch hands that policy.
//
// The package may import bots, seat, searchseat, botpolicy, view and rules,
// and never host or testkit/testutil (archtest pins both): the host's
// ShadowScan calls it, and the offline triage tool recomputes B through it,
// so the answer must not depend on a host.
package shadow

import (
	"context"
	"errors"
	"fmt"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// Answer returns the intent policy would give at e's pending decision, built
// exactly as the host's dispatch (server/host/match.go projectNextData) builds that
// seat's input, in the same order: an EnvSeat that WantsEnv gets an Env
// whose honest root is searchseat.HonestRoot over feed with deal seed
// bots.RootSeed(seed, d.Seq); else a BoardSeat gets the botpolicy.Board;
// else a plain seat gets the seat's projected View.
//
// A fresh bots.New instance is built for every call, with
// DecisionDeadlineMS 0: a wall-clock bail-out would make two runs diverge,
// and heuristic.Bot's only cross-decision state is its RNG, which the per-decision
// seed replaces. B is therefore a pure function of (e, feed, policy, seed).
//
// e must be a replay walk's engine at an intent boundary, never the live
// engine. feed is the deciding seat's observation stream captured through
// this boundary (nil is allowed for a policy that never asks for an Env: an
// Env decision with a nil feed plays the policy's refusal fallback, as a
// stopped live feed does). The payment extension is ensured when the policy
// consumes payment plans or autoPayMana is set, as the host does; that is a
// pure derived read of e, and Answer changes nothing else about it.
func Answer(e *rules.Engine, feed *searchseat.Feed, setup searchprobe.PublicGame,
	policy string, seed uint64, autoPayMana bool, deps bots.Deps) (decision.Intent, error) {
	if e == nil {
		return decision.Intent{}, errors.New("shadow: nil engine")
	}
	d := e.Pending()
	if d == nil {
		return decision.Intent{}, errors.New("shadow: no pending decision")
	}
	s, err := bots.New(policy, bots.Options{Seed: seed, AutoPayMana: autoPayMana, Deps: deps, DecisionDeadlineMS: 0})
	if err != nil {
		return decision.Intent{}, fmt.Errorf("shadow: %w", err)
	}
	wantsPlans := autoPayMana
	if c, ok := s.(seat.PaymentPlanConsumer); ok && c.WantsPaymentActions() {
		wantsPlans = true
	}
	if wantsPlans {
		e.EnsurePaymentActions()
	}
	dc := *e.Pending().Clone()
	p := dc.Player
	ctx := context.Background()

	if es, ok := s.(bots.EnvSeat); ok && es.WantsEnv(&dc) {
		var brd botpolicy.Board
		b := botpolicy.BoardFromGameInto(e.G, e, p, &brd)
		v := view.ProjectForControlled(e.G, e, p, view.Seat, controlledSeats(e.G, p), &dc)
		v.Round = view.RoundOf(e.G, e.L.Events)
		root, reason := searchseat.HonestRoot(setup, feed, e, bots.RootSeed(seed, dc.Seq))
		env := bots.Env{
			View:        v,
			Board:       b,
			Search:      searchseat.Env{Setup: setup, Engine: root, Board: b, Feed: feed},
			RootRefused: reason,
		}
		return es.DecideEnv(ctx, env, dc)
	}
	if bs, ok := s.(heuristic.BoardSeat); ok {
		var brd botpolicy.Board
		return bs.DecideBoard(ctx, botpolicy.BoardFromGameInto(e.G, e, p, &brd), dc)
	}
	v := view.ProjectForControlled(e.G, e, p, view.Seat, controlledSeats(e.G, p), &dc)
	v.Round = view.RoundOf(e.G, e.L.Events)
	return s.Decide(ctx, v, dc)
}

// controlledSeats mirrors host's helper of the same name: the live CR 723
// controlled seat the viewer answers for (state.Game.LiveController), nil
// otherwise (cr-g57).
func controlledSeats(g *state.Game, viewer state.PlayerID) []state.PlayerID {
	if ctl, ok := g.LiveController(g.Active); ok && ctl == viewer {
		return []state.PlayerID{g.Active}
	}
	return nil
}
