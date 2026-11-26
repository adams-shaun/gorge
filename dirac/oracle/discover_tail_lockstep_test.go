package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Hit the Mother Lode is Discover 10 whose Treasure tail (TokenAmount$
// Remembered$CardManaCost/NMinus.10, the mana-value shortfall) and Cleanup
// follow the free cast in the resolution's program. gorge runs the tail only
// after the cast election settles (effDiscover's suspended effPlay); the
// kernel parks the resolution at the election and resumes it when the cast
// queue drains (act_discoverfreecast.go). These games play both branches in
// lockstep: the cast branch casts the found card without paying its mana
// cost, the decline branch puts it into the owner's hand; the tail then reads
// the remembered card out of exile no longer (on the stack, or in hand) and
// mints the shortfall in tapped Treasures (10 - its mana value).

// discoverBranchPolicy answers every play election (gorge's KModes ask with
// ResumeKind "play"; in these decks only a discover's free cast) by branch:
// cast plays the found card, decline closes the ask with ActDone. At priority
// it ramps (a land play) and casts a Hit the Mother Lode when one is offered,
// so the discover happens early; every other decision goes to the seeded
// random policy.
func discoverBranchPolicy(seed uint64, cast bool, tallies *[3]int, mu *sync.Mutex) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(ctx *PickCtx) Canon {
		if d := ctx.G.E.Pending(); d != nil && d.Kind == decision.KModes && d.ResumeKind == "play" {
			for _, o := range ctx.D.Options {
				if cast && o.Kind() == ActMode {
					mu.Lock()
					tallies[0]++
					mu.Unlock()
					return o
				}
				if !cast && o.Kind() == ActDone {
					mu.Lock()
					tallies[1]++
					mu.Unlock()
					return o
				}
			}
		}
		for _, o := range ctx.D.Options {
			if o.Kind() == ActPlayLand {
				return o
			}
			if o.Kind() == ActCast && optionFaceName(ctx, o) == "Hit the Mother Lode" {
				return o
			}
		}
		return rnd.Pick(ctx)
	})
}

func TestMotherLodeDiscoverTailLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Hit the Mother Lode", Count: 4}, {Name: "Savannah Lions", Count: 12},
		{Name: "Plains", Count: 18}, {Name: "Mountain", Count: 6},
	}
	b := []cardset.Entry{
		{Name: "Savannah Lions", Count: 12}, {Name: "Plains", Count: 24}, {Name: "Mountain", Count: 4},
	}
	set := printedDelveSet(t, [2][]string{{"Hit the Mother Lode", "Savannah Lions", "Plains", "Mountain"}, {"Savannah Lions", "Plains", "Mountain"}})
	reg := synthReg(t)
	var cs [2][]*cards.Card
	for s, d := range [2][]cardset.Entry{a, b} {
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
	tallies := [3]int{} // cast, decline, treasures at game end
	seedChunks(t, 1, 30, 15, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		for seed := lo; seed <= hi; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			tbl, _, _, err := set.Matchup(a, b)
			if err != nil {
				t.Fatal(err)
			}
			m, err := NewMatchup(reg, [2]string{"motherlode-a", "motherlode-b"}, cs, 0)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40}
			rc.Finish = func(g *GorgeSide) {
				n := 0
				for p := state.PlayerID(0); p < 2; p++ {
					for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
						if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Treasure Token" {
							n++
						}
					}
				}
				mu.Lock()
				tallies[2] += n
				mu.Unlock()
			}
			pol := discoverBranchPolicy(seed, seed%2 == 0, &tallies, mu)
			res := Lockstep(m, seed, k, pol, rc)
			CloseSide(k)
			if d := res.Div; d != nil {
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
			}
		}
	})
	if tallies[0] == 0 || tallies[1] == 0 {
		t.Fatalf("the branches were not both played: cast %d, decline %d", tallies[0], tallies[1])
	}
	if tallies[2] == 0 {
		t.Fatalf("no Treasure survived to a game end: the tail's shortfall never minted")
	}
}
