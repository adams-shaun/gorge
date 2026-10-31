package host

import (
	"context"
	"fmt"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/divergence"
	"github.com/adams-shaun/gorge/server/divergence/shadow"
)

// ShadowScan is HD-3 of the human-divergence design
// (docs/design/2026-10-10-human-divergence-design.md §5.3-§5.5):
// it replays match k of table id once and, at every decision the seat
// answered deliberately, asks a fresh shadow instance of policy what it would
// have answered (shadow.Answer). Every material difference (divergence.Material)
// is returned as a divergence-v1 record, sorted by intent.
//
// A decision is scanned only when the seat owned it, its intent index is
// inside the log, and the provenance ledger marks it neither caretaker nor
// client-auto (HD-2): those were not a human's choice. The scan copies
// (cfg, log, ledger) under the match lock exactly as SnapshotForFeedback
// does and runs the walk outside it, so it never drives the live engine and
// blocks a Submit only for the copies.
//
// When policy is an EnvSeat the scan keeps one searchseat.Feed for seat,
// observing at every boundary and recording the seat's logged answers (the
// RebuildFeed loop), so the shadow's honest root is built from the same
// observation the live host would have given that seat. For a search policy
// each shadow decision runs under the registry's search-slot gate, so the
// scan never competes with live tables beyond Options.SearchSlots.
//
// The shadow seed of intent n is divergence.ShadowSeed(match seed, seat, n)
// and is recorded on the divergence, so triage can recompute B. Nothing is
// submitted anywhere: the result is host-side metadata, never replay input.
func (r *Registry) ShadowScan(id TableID, k int, seat state.PlayerID, policy string) ([]divergence.Divergence, error) {
	divs, _, err := r.shadowScan(id, k, seat, policy)
	return divs, err
}

// shadowScan is ShadowScan plus the number of decisions it compared, the
// precondition the parity test checks so that "0 divergences" cannot be a
// scan that visited nothing.
func (r *Registry) shadowScan(id TableID, k int, seat state.PlayerID, policy string) ([]divergence.Divergence, int, error) {
	t, m, err := r.lookup(id, k)
	if err != nil {
		return nil, 0, fmt.Errorf("host: table %s match %d: %w", id, k, err)
	}
	pol, err := bots.Normalize(policy)
	if err != nil {
		return nil, 0, fmt.Errorf("host: shadow scan: %w", err)
	}
	entry, _ := bots.Lookup(pol)

	m.mu.RLock()
	if int(seat) >= len(m.gc.Decks) {
		n := len(m.gc.Decks)
		m.mu.RUnlock()
		return nil, 0, fmt.Errorf("host: table %s: seat %d out of range (%d seats)", id, seat, n)
	}
	l := m.e.L.Clone()
	ledger := append([]uint8(nil), m.ledger...)
	cfg := gorgegame.RulesConfig(m.gc)
	seed := m.seed
	m.mu.RUnlock()
	if len(ledger) < len(l.Intents) {
		return nil, 0, fmt.Errorf("host: table %s match %d: provenance ledger holds %d entries for %d intents", id, k, len(ledger), len(l.Intents))
	}

	var gate *searchSlots
	if entry.Search {
		gate = r.searchGate
	}
	autoPay := t.cfg.autoPayManaEnabled()
	setup := searchprobe.PublicGame{Names: cfg.Names, Decks: cfg.Decks, Tokens: cfg.Tokens, StartingLife: cfg.StartingLife}
	var feed *searchseat.Feed
	if entry.Env {
		feed = searchseat.NewFeed(seat)
	}
	ctx := context.Background()

	var out []divergence.Divergence
	scanned := 0
	n := len(l.Intents)
	_, err = replay.Walk(l, cfg, n, func(e *rules.Engine, i int) error {
		if feed != nil {
			feed.Observe(e) // a stopped feed plays the policy's fallback, as live
		}
		if i >= n {
			return nil
		}
		d := e.Pending()
		if d == nil || d.Player != seat {
			return nil
		}
		if ledger[i]&(provCaretaker|provAuto) == 0 {
			scanned++
		}
		in := l.Intents[i]
		if in.Payment != nil || in.Announce != nil {
			e.EnsurePaymentActions()
		}
		if ledger[i]&(provCaretaker|provAuto) == 0 {
			dv, err := r.shadowDecision(ctx, gate, t.stop, e, feed, setup, pol, seed, autoPay, r.opts.BotDeps, seat, i, in, d)
			if err != nil {
				return fmt.Errorf("intent %d: %w", i, err)
			}
			if dv != nil {
				dv.Table, dv.Match = string(id), k
				dv.ID = fmt.Sprintf("%s-m%d/%d", id, k, i)
				out = append(out, *dv)
			}
		}
		if feed != nil {
			if err := feed.RecordAnswer(d, in); err != nil {
				return fmt.Errorf("record answer at intent %d: %w", i, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, 0, fmt.Errorf("host: shadow scan of %s match %d: %w", id, k, err)
	}
	return out, scanned, nil
}

// shadowDecision computes the shadow answer at one boundary and returns the
// divergence record when it differs materially from the human's logged
// answer a, or nil when it does not.
func (r *Registry) shadowDecision(ctx context.Context, gate *searchSlots, stop <-chan struct{},
	e *rules.Engine, feed *searchseat.Feed, setup searchprobe.PublicGame, policy string,
	matchSeed uint64, autoPay bool, deps bots.Deps, seat state.PlayerID, n int,
	a decision.Intent, d *decision.Decision) (*divergence.Divergence, error) {
	// The decision as the human saw it, captured before the shadow's own
	// payment-plan build can change the engine's pending copy.
	dh := *d.Clone()
	shadowSeed := divergence.ShadowSeed(matchSeed, seat, n)
	if gate != nil {
		if err := gate.acquire(ctx, stop); err != nil {
			return nil, err
		}
		defer gate.release()
	}
	b, err := shadow.Answer(e, feed, setup, policy, shadowSeed, autoPay, deps)
	if err != nil {
		return nil, err
	}
	// Judge against the engine's pending decision after the shadow's build:
	// the logged answer's payment witness indexes the extension.
	dm := e.Pending()
	material, why := divergence.Material(dm, a, b)
	if !material {
		return nil, nil
	}
	before := uint64(0)
	if ev := len(e.L.Events); ev > 0 {
		before = uint64(ev - 1)
	}
	opts := append([]decision.Option(nil), dh.Options...)
	return &divergence.Divergence{
		RecordType:    divergence.RecordTypeDivergence,
		SchemaVersion: divergence.DivergenceSchemaVersion,
		Seat:          seat,
		Intent:        n,
		EventSeq:      before,
		DecisionSeq:   dh.Seq,
		Turn:          int(e.G.Turn),
		Kind:          dh.Kind,
		Policy:        policy,
		ShadowSeed:    shadowSeed,
		Options:       opts,
		Human:         divergence.Answer{Choices: nonNilInts(a.Choices), Label: divergence.ChoiceSummary(dm, a)},
		Shadow:        divergence.Answer{Choices: nonNilInts(b.Choices), Label: divergence.ChoiceSummary(dm, b)},
		Material:      why,
	}, nil
}

func nonNilInts(s []int) []int {
	if s == nil {
		return []int{}
	}
	return append([]int(nil), s...)
}
