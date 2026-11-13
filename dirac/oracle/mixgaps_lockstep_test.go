package oracle

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// White Lotus Hideout is one permanent with three mana abilities: a {T} {C}
// unit, a restricted {T} one-of-any-colour (Lesson and Shrine spells only)
// and a {1},{T} one-of-any-colour filter. gorge's census lists only the {C};
// its prefix search activates the other two as the gaps of one object. The
// kernel plans the slot as both an anyR gap and a filter gap
// (dirac mana_mixsrc.go). Each matchup plays 150 seeds, the two option
// surfaces alternating, and asserts the payments it is about happened.
func TestMixGapsCombinedSourceLockstep(t *testing.T) {
	t.Parallel() // its subtests run isolated, so the parent need not hold the serial phase
	lessonB := []cardset.Entry{{Name: "Firebending Lesson", Count: 8}, {Name: "Appa, Loyal Sky Bison", Count: 4}, {Name: "Island", Count: 12}, {Name: "Plains", Count: 16}}
	for _, mu := range []struct {
		name               string
		a, b               []cardset.Entry
		restricted, filter bool // the matchup must see these Hideout abilities activated
	}{
		{"lesson_and_other",
			[]cardset.Entry{{Name: "White Lotus Hideout", Count: 2}, {Name: "Earthbending Lesson", Count: 8}, {Name: "Ran and Shaw", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 18}},
			lessonB, true, true},
		{"other_only",
			[]cardset.Entry{{Name: "White Lotus Hideout", Count: 2}, {Name: "Ran and Shaw", Count: 8}, {Name: "The Fire Nation Drill", Count: 8}, {Name: "Mountain", Count: 12}, {Name: "Plains", Count: 10}},
			[]cardset.Entry{{Name: "White Lotus Hideout", Count: 2}, {Name: "Ran and Shaw", Count: 8}, {Name: "The Fire Nation Drill", Count: 8}, {Name: "Island", Count: 12}, {Name: "Plains", Count: 10}},
			false, true},
		{"lesson_only",
			[]cardset.Entry{{Name: "White Lotus Hideout", Count: 2}, {Name: "Airbending Lesson", Count: 8}, {Name: "Earthbending Lesson", Count: 8}, {Name: "Plains", Count: 22}},
			lessonB, true, false},
	} {
		t.Run(mu.name, func(t *testing.T) {
			if isolate(t) {
				return
			}
			st := mixSrcLockstep(t, [2][]cardset.Entry{mu.a, mu.b}, 150)
			t.Logf("%s: %+v", mu.name, st)
			if st.inPlayUntapped == 0 || st.bothUntapped == 0 {
				t.Fatalf("no decision saw a Hideout untapped on the battlefield (both: %d)", st.bothUntapped)
			}
			if mu.restricted && st.restrictedCasts == 0 {
				t.Fatal("no Lesson cast was paid through the restricted any-colour ability")
			}
			if mu.filter && st.filterCasts == 0 {
				t.Fatal("no cast was paid through the {1},{T} filter")
			}
			if !mu.restricted && st.restricted != 0 {
				t.Fatalf("the restricted ability was activated %d times with no Lesson in the decks", st.restricted)
			}
			if st.bothPaid == 0 {
				t.Fatal("no cast was paid with both Hideouts")
			}
		})
	}
}

type mixSrcStats struct {
	inPlayUntapped, bothUntapped int // decisions with one / both of a player's Hideouts untapped
	restricted, filter           int // Hideout ability activations, by ability
	restrictedCasts, filterCasts int // casts whose payment activated them
	bothPaid                     int // casts whose payment tapped both of a player's Hideouts
}

// mixSrcLockstep is fixtureLockstep over the std-tla pool with the
// Hideout's payments counted from gorge's log.
func mixSrcLockstep(t *testing.T, decks [2][]cardset.Entry, seeds int) mixSrcStats {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-tla")
	var cs [2][]*cards.Card
	for s, d := range decks {
		n := 0
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d has %d cards", s, n)
		}
	}
	// The seeds are independent games, so they play on a few goroutines, each with its own table;
	// the counts are sums, and a divergence reports the lowest divergent seed, as a serial run would.
	workers := min(4, runtime.GOMAXPROCS(0), seeds)
	tbls := make([]*kop.Table, workers)
	for w := range tbls {
		tbl, _, _, err := set.Matchup(decks[0], decks[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[w] = tbl
	}
	var (
		mu    sync.Mutex
		st    mixSrcStats
		fails = map[uint64]string{}
		wg    sync.WaitGroup
	)
	next := make(chan uint64)
	for _, tbl := range tbls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for seed := range next {
				s, fail := mixSrcSeed(reg, set, tbl, cs, seed)
				mu.Lock()
				st.add(s)
				if fail != "" {
					fails[seed] = fail
				}
				mu.Unlock()
			}
		}()
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		next <- seed
	}
	close(next)
	wg.Wait()
	if len(fails) > 0 {
		low := uint64(0)
		for seed := range fails {
			if low == 0 || seed < low {
				low = seed
			}
		}
		t.Fatal(fails[low])
	}
	return st
}

func (s *mixSrcStats) add(o mixSrcStats) {
	s.inPlayUntapped += o.inPlayUntapped
	s.bothUntapped += o.bothUntapped
	s.restricted += o.restricted
	s.filter += o.filter
	s.restrictedCasts += o.restrictedCasts
	s.filterCasts += o.filterCasts
	s.bothPaid += o.bothPaid
}

// mixSrcSeed plays one seed of mixSrcLockstep and returns its counts, and a failure message for a
// game that could not be built or diverged.
func mixSrcSeed(reg *cards.Registry, set cardset.Set, tbl *kop.Table, cs [2][]*cards.Card, seed uint64) (mixSrcStats, string) {
	var st mixSrcStats
	cfg := Config{GorgeAutopaySurface: seed%2 == 0}
	var acts []mixSrcAct
	m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
	if err != nil {
		return st, err.Error()
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		return st, err.Error()
	}
	res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
		Check: func(ref *GorgeSide, _ Side) *Divergence {
			if ref.E.ManaAbilityHook == nil {
				e := ref.E
				e.ManaAbilityHook = func(p state.PlayerID, src state.ObjID, sa *cards.SA) {
					if o := e.G.Obj(src); o != nil && isHideout(o) {
						acts = append(acts, mixSrcAct{at: len(e.L.Events), p: int(p) & 1, kind: hideoutAbility(sa)})
					}
				}
			}
			for p := 0; p < 2; p++ {
				n := 0
				for _, id := range ref.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
					if o := ref.E.G.Obj(id); o != nil && isHideout(o) && !o.Tapped && o.Controller == state.PlayerID(p) {
						n++
					}
				}
				if n > 0 {
					st.inPlayUntapped++
				}
				if n > 1 {
					st.bothUntapped++
				}
			}
			return nil
		},
		Finish: func(g *GorgeSide) { mixSrcScan(g, acts, &st) }})
	CloseSide(k)
	if d := res.Div; d != nil {
		return st, fmt.Sprintf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
	}
	return st, ""
}

func isHideout(o *state.Object) bool {
	return o.Face() != nil && o.Face().Name == "White Lotus Hideout"
}

// mixSrcAct is one Hideout mana ability activation: the log length when it
// began, its player, and which ability (0 the {C}, 1 the restricted
// any-colour, 2 the {1},{T} filter).
type mixSrcAct struct{ at, p, kind int }

func hideoutAbility(sa *cards.SA) int {
	switch {
	case sa.Params["RestrictValid"] != "":
		return 1
	case sa.Params["Produced"] == "Any":
		return 2
	}
	return 0
}

// mixSrcScan walks a finished game's log: each Hideout ability activation is
// charged to its player's last spell put on the stack in the same step (CR
// 601.2a precedes the payment, 601.2g-h).
func mixSrcScan(g *GorgeSide, acts []mixSrcAct, st *mixSrcStats) {
	type cast struct {
		restr, filt bool
		hide        int
	}
	var cur [2]*cast
	var all []*cast
	for i := range g.E.L.Events {
		for len(acts) > 0 && acts[0].at <= i {
			a := acts[0]
			acts = acts[1:]
			c := cur[a.p]
			if c == nil {
				continue
			}
			c.hide++
			switch a.kind {
			case 1:
				st.restricted++
				c.restr = true
			case 2:
				st.filter++
				c.filt = true
			}
		}
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.PutOnStack:
			c := &cast{}
			cur[int(ev.Player)&1] = c
			all = append(all, c)
		case events.StepChange:
			cur = [2]*cast{}
		}
	}
	for _, c := range all {
		if c.restr {
			st.restrictedCasts++
		}
		if c.filt {
			st.filterCasts++
		}
		if c.hide > 1 {
			st.bothPaid++
		}
	}
}
