package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestMarksCoverFDN is the kernel's marks check (dirac.CheckMarks) over
// random FDN games: after every action, each object without a change mark
// hashes as it did when the epoch was set, the epoch re-based every few
// actions. FDN cards reach mutation sites the vanilla table does not. Each
// posed decision is stepped twice: a re-step must pose it unchanged.
func TestMarksCoverFDN(t *testing.T) {
	t.Parallel()
	c, pool := testCorpus(t)
	marksCover(t, c, pool)
}

// TestMarksCoverDealtFDN is the same check on decks dealt from the set: a counter
// cost's pick, posed and stepped again, must leave the build as it was.
func TestMarksCoverDealtFDN(t *testing.T) {
	t.Parallel()
	c0, _ := testCorpus(t)
	c := &Corpus{Reg: c0.Reg, decks: map[string][]*cards.Card{}}
	pool, err := addSynthFDNDecks(c)
	if err != nil {
		t.Fatal(err)
	}
	marksCover(t, c, pool)
}

func marksCover(t *testing.T, c *Corpus, pool []PoolDeck) {
	t.Helper()
	var base [dirac.MaxObj]uint64
	games, actions, xs := 0, 0, 0
	for _, p := range Pairings(pool, 200, 31) {
		m, tbl, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		side, err := RealKernel(tbl, nil, Config{PassElision: true})(m, p.Seed)
		if err != nil {
			continue
		}
		r := side.(*realSide)
		rng := rand.New(rand.NewPCG(p.Seed, 7))
		every := 1 + int(p.Seed%4)
		games++
		func() {
			defer func() {
				if x := recover(); x != nil {
					t.Errorf("%s vs %s: kernel panic: %v", p.A, p.B, x)
				}
			}()
			for i := 0; i < 3000; i++ {
				r.s.StepInto(&r.d)
				if r.d.Kind == dirac.DkNone {
					return
				}
				// stepping a posed decision again poses the same decision (a
				// search steps its root after the caller did)
				if r.d.Kind == dirac.DkChoose && r.d.Sub == dirac.ChX {
					xs++
				}
				h := r.s.DecisionHash(&r.d)
				var d2 dirac.Decision
				if r.s.StepInto(&d2); r.s.DecisionHash(&d2) != h {
					t.Fatalf("%s vs %s action %d: a re-step posed a different decision", p.A, p.B, i)
				}
				if i%every == 0 {
					r.s.SetEpoch(uint32(i + 1))
					for o := 0; o < r.s.NumObjs(); o++ {
						base[o] = r.s.ObjHash(dirac.Obj(o))
					}
				}
				r.s.Apply(r.d.Options[rng.IntN(int(r.d.N))])
				actions++
				if o, ok := dirac.CheckMarks(&r.s, &base); !ok {
					t.Fatalf("%s vs %s action %d: object %d (card %d) changed without a mark", p.A, p.B, i, o, r.s.ObjCard(o))
				}
			}
		}()
	}
	t.Logf("%d FDN games, %d actions checked, %d X asks re-stepped", games, actions, xs)
	if games == 0 {
		t.Skip("no FDN matchup built")
	}
}
