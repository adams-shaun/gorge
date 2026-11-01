package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFetchNeedPicksTheNeededBasic: at Evolving Wilds' library searches,
// off the bot takes the first offer in library order; with :fetchneed it
// takes a land covering the most unmet colour pips (basic, then fewest
// colours, then library order on ties), and the two differ somewhere.
func TestFetchNeedPicksTheNeededBasic(t *testing.T) {
	a := []cardset.Entry{
		{Name: "Evolving Wilds", Count: 10},
		{Name: "Forest", Count: 8},
		{Name: "Island", Count: 8},
		{Name: "Mountain", Count: 6},
		{Name: "Llanowar Elves", Count: 8},
		{Name: "Think Twice", Count: 10},
		{Name: "Burst Lightning", Count: 10},
	}
	b := []cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Burst Lightning", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(a, b)
	if err != nil {
		t.Fatal(err)
	}
	f := NewFacts(tbl)
	off, on := New(f), New(f).EnableFetchNeed()
	if off.FetchNeed() || !on.FetchNeed() {
		t.Fatal("FetchNeed flag")
	}
	hits, differ := 0, 0
	for seed := uint64(1); seed <= 80 && hits < 40; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{})
		pol := dirac.NewRandomPolicy(seed ^ 0x66657463)
		var d dirac.Decision
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkChoose && d.Sub == dirac.ChGenSearch && d.N > 1 {
				i0, i1 := off.Pick(s, &d), on.Pick(s, &d)
				if i0 != off.firstObj(s, &d) {
					t.Fatalf("off: pick %d is not the first offer", i0)
				}
				on.b.fill(s, f, d.Player)
				unmet := on.b.unmetNeed()
				cover := func(i int) int32 {
					c, _, _ := landScore(f.card(s.ObjCard(dirac.Obj(d.Options[i].Source()))), false, &unmet)
					return c
				}
				if d.Options[i1].Kind() == dirac.AkDone {
					t.Fatal("on: took Done")
				}
				for j := 0; j < int(d.N); j++ {
					if d.Options[j].Kind() != dirac.AkDone && cover(j) > cover(i1) {
						t.Fatalf("on: option %d covers %d > the pick's %d", j, cover(j), cover(i1))
					}
				}
				hits++
				if s.ObjCard(dirac.Obj(d.Options[i0].Source())) != s.ObjCard(dirac.Obj(d.Options[i1].Source())) {
					differ++
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	if hits < 10 || differ == 0 {
		t.Fatalf("%d library searches, %d answered differently: the walk found too few", hits, differ)
	}
}
