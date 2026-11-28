package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// dcFourSeatVisit is gorge's visit order for seat 2's Disorienting Choice in
// a four-seat pod: AliveFrom(controller) without the controller, so turn
// order from seat 2 and not seat order.
var dcFourSeatVisit = []state.PlayerID{3, 0, 1}

// dcFourSeat returns a player of four-seat pods on the fixture pool in which
// seat 2 holds Disorienting Choice and every other seat an Ornithopter, one
// seed per call. The policy casts the spell, names every offered Ornithopter
// and keeps each; asked lists the players asked the per-permanent choice, in
// order. mutate, when non-nil, rewrites the table first. A seat in bare holds
// Forests only, so it has nothing the spell can target.
func dcFourSeat(t *testing.T, mutate func(*kop.Table), bare ...state.PlayerID) func(seed uint64) (res Result, asked []state.PlayerID) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("repeatplayerssel")
	if err != nil {
		t.Fatal(err)
	}
	thopter := append(deckOf(1, "Ornithopter"), deckOf(39, "Forest")...)
	caster := append(deckOf(1, "Disorienting Choice"), deckOf(39, "Forest")...)
	decks := [4][]string{thopter, thopter, caster, thopter}
	for _, p := range bare {
		decks[p] = deckOf(40, "Forest")
	}
	var lists [][]cardset.Entry
	for _, d := range decks {
		es := make([]cardset.Entry, len(d))
		for i, n := range d {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		lists = append(lists, es)
	}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	target := rules.StartSeat{Battlefield: []int{0, 1}, Hand: []int{2, 3}}
	start := []rules.StartSeat{target, target, {Battlefield: []int{1, 2, 3, 4, 5}, Hand: []int{0, 6}}, target}
	return func(seed uint64) (Result, []state.PlayerID) {
		r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{decks[0], decks[1]},
			MoreDecks: []string{"x2", "x3"}, MoreCards: [][]string{decks[2], decks[3]}, Seed: seed, PassElision: true, Surface: "full"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		m.Start = &rules.StartSpec{Seats: start}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 1500, 6
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		defer CloseSide(k)
		g, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		var asked []state.PlayerID
		base := castAlwaysPolicy(seed, map[string]bool{"Disorienting Choice": true})
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				for _, o := range c.D.Options {
					oa, ok := c.G.GorgeOption(o)
					if !ok {
						continue
					}
					if oa.Kind == "mode" && oa.Label == "Keep targeted card" {
						asked = append(asked, state.PlayerID(c.D.Player))
						return o
					}
					if c.D.Kind == DTarget && oa.Obj != 0 {
						if ob := c.G.E.G.Obj(oa.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Ornithopter" {
							return o
						}
					}
				}
			}
			return base.Pick(c)
		})
		res := LockstepLed(m, seed, g, k, pol, rc)
		return res, asked
	}
}

// requireDCFourSeatVisit: seat 2's Disorienting Choice loops over the three
// opponents whose permanents it chose, in gorge's order, with no divergence.
func requireDCFourSeatVisit(t *testing.T) {
	t.Helper()
	play := dcFourSeat(t, nil)
	for seed := uint64(1); seed <= 4; seed++ {
		res, asked := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if !slices.Equal(asked, dcFourSeatVisit) {
			t.Errorf("seed %d: Disorienting Choice asked %v, want %v", seed, asked, dcFourSeatVisit)
		}
	}
}

// requireDCControllerMutationDiverges: with Disorienting Choice's loop over
// the targets' controllers swapped for the caster alone, a game diverges.
func requireDCControllerMutationDiverges(t *testing.T) {
	t.Helper()
	play := dcFourSeat(t, func(tbl *kop.Table) {
		n := mutateCard(t, tbl, "Disorienting Choice", func(_ *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpRepeatEachPlayer && in.Obj == kop.RefTgtControllers {
				in.Obj = kop.RefController
			}
		})
		if n == 0 {
			t.Fatal("Disorienting Choice: no RepeatEachPlayer over RefTgtControllers in the table")
		}
	})
	for seed := uint64(1); seed <= 4; seed++ {
		if res, _ := play(seed); res.Div != nil {
			return
		}
	}
	t.Error("Disorienting Choice's loop over the controller alone changed no game")
}
