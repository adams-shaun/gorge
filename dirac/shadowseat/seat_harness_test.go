package shadowseat

import (
	"context"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/vanilla"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/bench"
)

// pool is one card pool a test plays: its table, the set it was generated
// from (nil for vanilla) and how a deck is drawn.
type pool struct {
	tbl  *kop.Table
	set  cardset.Set
	deck func(r *rand.Rand) []kop.CardID
}

// vanillaPool is the staging verifier's pool (dirac/shadow stage_test.go
// stageDeck): 17 basic lands, 20 vanilla or keyword creatures and 3 Hungry
// Ghoul.
func vanillaPool() *pool {
	var lands, creatures []kop.CardID
	for id := vanilla.Plains; id <= vanilla.Forest; id++ {
		lands = append(lands, id)
	}
	for id := vanilla.SavannahLions; id <= vanilla.TundraWall; id++ {
		creatures = append(creatures, id)
	}
	return &pool{tbl: vanilla.Table, deck: func(r *rand.Rand) []kop.CardID {
		var d []kop.CardID
		for i := 0; i < 17; i++ {
			d = append(d, lands[r.IntN(len(lands))])
		}
		for i := 0; i < 20; i++ {
			d = append(d, creatures[r.IntN(len(creatures))])
		}
		return append(d, vanilla.HungryGhoul, vanilla.HungryGhoul, vanilla.HungryGhoul)
	}}
}

// gameSetup is one game's engine config and each seat's deck list.
type gameSetup struct {
	cfg   rules.Config
	lists [][]string
}

// setup draws game seed's two decks from the pool.
func (p *pool) setup(t *testing.T, reg *cards.Registry, seed uint64) gameSetup {
	t.Helper()
	r := rand.New(rand.NewPCG(seed, 0x5748))
	decks := [2][]kop.CardID{p.deck(r), p.deck(r)}
	m, err := oracle.KernelMatchup(reg, p.tbl, [2]string{"a", "b"}, decks, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var lists [][]string
	for s := range decks {
		var l []string
		for _, id := range decks[s] {
			l = append(l, p.tbl.Cards[id].Name)
		}
		lists = append(lists, l)
	}
	return gameSetup{cfg: m.Config(seed), lists: lists}
}

// newSeat is the shadow seat of game g with the heuristic bot as its fallback.
func (p *pool) newSeat(t *testing.T, reg *cards.Registry, g gameSetup, sims, worlds int, seed uint64) *Seat {
	t.Helper()
	s, err := New(Config{
		Tbl: p.tbl, Set: p.set, Reg: reg, Lists: g.lists, Sims: sims, Worlds: worlds,
		Seed: seed, Fallback: heuristic.NewBot(seed ^ 0xfa11),
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// recorder wraps a seat and keeps the intents it answers.
type recorder struct {
	seat.Seat
	intents []decision.Intent
	before  func(v *view.View, d *decision.Decision)
}

func (r *recorder) WantsPaymentActions() bool { return true }

func (r *recorder) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if r.before != nil {
		r.before(&v, &d)
	}
	in, err := r.Seat.Decide(ctx, v, d)
	r.intents = append(r.intents, in)
	return in, err
}

const (
	maxTurns   = 60
	maxIntents = 4000
)

// play plays g with the shadow seat in seat index shadowIdx against the
// heuristic bot and returns the outcome.
func play(t *testing.T, g gameSetup, shadowIdx int, sd seat.Seat, seed uint64, hooks bench.Hooks) bench.Outcome {
	t.Helper()
	seats := make([]seat.Seat, 2)
	seats[shadowIdx] = sd
	seats[1-shadowIdx] = heuristic.NewBot(seed ^ 0xb07)
	o, _, err := bench.PlayGame(g.cfg, seats, maxTurns, maxIntents, hooks)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	return o
}
