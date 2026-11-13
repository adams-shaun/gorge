//go:build dirac_mp

package search

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSearchPlaysFourSeatPods: every seat of a four-seat pod searches under
// the leaves and deals that read the opponents (the heuristic and MageZero
// leaves' strongest opponent, the opponents' points, the hidden-information
// re-deal, the information seed, the clairvoyant deal), seats 2 and 3 among
// them, where a two-seat read (1 - seat) wrapped or indexed past the seats.
func TestSearchPlaysFourSeatPods(t *testing.T) {
	type cfg struct {
		name string
		opts func() Options
		ev   Evaluator
	}
	cfgs := []cfg{
		{"heur", DefaultOptions, HeuristicLeaf{}},
		{"mz", DefaultOptions, MZLeaf{}},
		{"oppnodes", func() Options { o := DefaultOptions(); o.OpponentNodes = true; return o }, HeuristicLeaf{}},
		{"infoseed", func() Options { o := DefaultOptions(); o.InfoSeed = true; return o }, MZLeaf{}},
		{"clair", func() Options { o := DefaultOptions(); o.Clairvoyant = true; return o }, Uniform{}},
	}
	for ci, c := range cfgs {
		opts := c.opts()
		opts.Sims = 24
		sr := NewSearcher(opts, c.ev, Heuristic{})
		var searched [dirac.MaxSeats]int
		var s dirac.State
		var decks [dirac.MaxSeats][]dirac.CardID
		for p := range decks {
			decks[p] = v.Decks[(ci+p)%len(v.Decks)]
		}
		s.InitN(v.Table, decks, 4, uint64(ci)+5, dirac.Config{})
		var d dirac.Decision
		for i := 0; i < 6000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			r := sr.Search(&s, uint64(i)*0x51+uint64(ci))
			if r.Stats.Searched > 0 {
				searched[d.Player]++
			}
			r.Play(&s)
		}
		for p := uint8(0); p < 4; p++ {
			if searched[p] == 0 {
				t.Errorf("%s: seat %d never searched (%v)", c.name, p, searched)
			}
		}
	}
}

// TestHeuristicLeafReadsStrongestOpponent: with more than two seats the
// leaf's opponent score is the strongest living opponent's (gorge azmcts
// heuristicLeafValue), so each seat's value is the sigmoid of its own score
// less the largest of the other three.
func TestHeuristicLeafReadsStrongestOpponent(t *testing.T) {
	var s dirac.State
	var decks [dirac.MaxSeats][]dirac.CardID
	for p := range decks {
		decks[p] = v.Decks[p%len(v.Decks)]
	}
	s.InitN(v.Table, decks, 4, 9, dirac.Config{})
	pol := dirac.NewRandomPolicy(4)
	var d dirac.Decision
	checked := 0
	for i := 0; i < 4000 && !s.Over(); i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if i%40 == 0 {
			var score [4]float64
			for p := uint8(0); p < 4; p++ {
				score[p] = float64(s.LifeOf(p) + 2*s.HandSize(p))
			}
			for o := 0; o < s.NumObjs(); o++ {
				ob := dirac.Obj(o)
				if s.ObjZone(ob) != kop.ZoneBattlefield {
					continue
				}
				if v.Table.Cards[s.ObjCard(ob)].Types&kop.TLand != 0 {
					score[s.Controller(ob)] += 3
					continue
				}
				pw, tg := s.PowerToughness(ob)
				m := 10.0
				if s.IsCreature(ob) {
					m += 2 * float64(pw+tg)
				}
				score[s.Controller(ob)] += m
			}
			for seat := uint8(0); seat < 4; seat++ {
				if s.FeatLost(seat) {
					continue
				}
				top := math.Inf(-1)
				for p := uint8(0); p < 4; p++ {
					if p != seat && !s.FeatLost(p) {
						top = max(top, score[p])
					}
				}
				want := float32(1 / (1 + math.Exp(-(score[seat]-top)/20)))
				if got := (HeuristicLeaf{}).Value(&s, &d, seat); got != want {
					t.Fatalf("step %d seat %d: leaf %v, want %v (scores %v)", i, seat, got, want, score)
				}
				checked++
			}
		}
		s.Apply(pol.Pick(&d))
	}
	if checked == 0 {
		t.Fatal("no position checked")
	}
}
