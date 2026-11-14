package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestTapChoiceLockstep plays, in the MBC draft pool holding them, cast
// routes on which gorge's oracle taps a lowered witness at priority, gorge
// leading. A kernel that tapped nothing there showed a different tapped flag
// and pool at the cast's first ask.
//
//   - a buyback {X} cast (Fanning the Flames): gorge's "buyback" potential
//     play is witnessed at the largest X the sources pay (rules
//     potentialModeCastPlan), tapped before X is announced (kernel beginCast).
//
// Every case must lower its spell through gorge's potential planner in some
// seed, in each of its modes with a witness that taps at least one source
// (the taps the kernel must mirror before the {X} ask), and no game may
// diverge.
func TestTapChoiceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cases := []struct {
		name    string
		decks   [2][]cardset.Entry
		lowered []string // cards gorge must lower in some seed
		modes   []string // play modes gorge must lower with a tapping witness in some seed
	}{
		{"buyback-x", [2][]cardset.Entry{
			{{Name: "Fanning the Flames", Count: 14}, {Name: "Mountain", Count: 26}},
			{{Name: "Fanning the Flames", Count: 14}, {Name: "Mountain", Count: 16}, {Name: "Swamp", Count: 10}},
		}, []string{"Fanning the Flames"}, []string{"buyback"}},
	}
	reg := synthReg(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			pool, set := tapChoicePool(t, c.decks)
			var cs [2][]*cards.Card
			for s, d := range c.decks {
				for _, e := range d {
					card, ok := reg.Lookup(e.Name)
					if !ok {
						t.Fatalf("%s not in the corpus", e.Name)
					}
					for i := 0; i < e.Count; i++ {
						cs[s] = append(cs[s], card)
					}
				}
			}
			tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
			if err != nil {
				t.Fatalf("%s: %v", pool, err)
			}
			lowered := map[string]int{}
			tapped := map[string]int{} // lowered plays by mode whose witness taps a source
			for seed := uint64(1); seed <= 40; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"tap-a", "tap-b"}, cs, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				inner := eagerPolicy(seed, map[string]int{})
				pol := PolicyFunc(func(pc *PickCtx) Canon {
					a := inner.Pick(pc)
					if g := pc.G; g != nil && g.fr != nil {
						for _, pe := range g.fr.prio {
							if pe.a == a && pe.kind == prLower {
								if o := g.E.G.Obj(pe.play.Obj); o != nil && o.Face() != nil {
									lowered[o.Face().Name]++
								}
								if pe.play.Mode != "" && len(pe.plan.Activations) > 0 {
									tapped[pe.play.Mode]++
								}
							}
						}
					}
					return a
				})
				res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
				CloseSide(k)
				if d := res.Div; d != nil {
					t.Fatalf("%s seed %d (%s): diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", pool, seed, SurfaceName(cfg), d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
				}
				if res.StallUnmodelled {
					t.Fatalf("%s seed %d: unmodelled stall %s", pool, seed, res.Stall)
				}
			}
			t.Logf("%s (%s): lowered casts %v, tapping witnesses by mode %v", c.name, pool, lowered, tapped)
			for _, n := range c.lowered {
				if lowered[n] == 0 {
					t.Errorf("gorge never lowered %s in 40 games", n)
				}
			}
			for _, md := range c.modes {
				if tapped[md] == 0 {
					t.Errorf("gorge never lowered a %s cast with a witness tapping a source in 40 games", md)
				}
			}
		})
	}
}

// tapChoicePool is the MBC draft pool that holds every card of decks, none
// refused.
func tapChoicePool(t *testing.T, decks [2][]cardset.Entry) (string, cardset.Set) {
	t.Helper()
	for _, name := range cardsettest.SynthNames(t) {
		if !strings.HasPrefix(name, cardset.SynthDraftPrefix+"mbc") {
			continue
		}
		s := cardsettest.Synth(t, name)
		all := true
		for _, d := range decks {
			for _, e := range d {
				i, ok := s.Lookup(e.Name)
				if !ok {
					all = false
					break
				}
				if s.Status(i) == cardset.StatusRefused {
					t.Fatalf("%s refuses %s: %s", name, e.Name, s.Reason(i))
				}
			}
		}
		if all {
			return name, s
		}
	}
	t.Fatalf("no MBC draft pool holds every card of %v", decks)
	return "", nil
}
