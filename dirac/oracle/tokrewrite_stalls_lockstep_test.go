package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A token copy that carries a rider, under Holo-Engineer's CreateToken
// rewriter. The rewriter replaces every copy mint with a plain Illusion, so
// the effect is left with no copy for TokenTapped$, PumpKeywords$ or
// AtEOT$ to act on (gorge continueCopyTokenProposal); the one AtEOT$
// Sacrifice group is still registered over no object and does nothing at the
// end step (CR 603.7). Tasha Yar's graveyard copy (AtEOT$ Sacrifice) and
// Blastoneuron's (PumpKeywords$ Haste with AtEOT$ Sacrifice) were the
// kernel's tokrewrite stall.
func TestTokRewriteStallsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Holo-Engineer", Count: 4}, {Name: "Tasha Yar, Out of Time", Count: 4},
		{Name: "Blastoneuron", Count: 4}, {Name: "MACO Strike Team", Count: 4},
		{Name: "Academy Instructor", Count: 3}, {Name: "Forest", Count: 10},
		{Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 5},
	}
	b := []cardset.Entry{
		{Name: "Holo-Engineer", Count: 3}, {Name: "Tasha Yar, Out of Time", Count: 4},
		{Name: "Blastoneuron", Count: 4}, {Name: "Replicator", Count: 3},
		{Name: "MACO Strike Team", Count: 4}, {Name: "Academy Instructor", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5},
	}
	decks := [2][]cardset.Entry{a, b}
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-trk")
	var cs [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	// the AtEOT$ Sacrifice groups registered over no object while the
	// copier's controller held a Holo-Engineer: the rewritten copy with a rider
	under := map[string]int{}
	evAt := 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g, evs := ref.E.G, ref.E.L.Events
		if len(evs) < evAt {
			evAt = 0
		}
		for j := evAt; j < len(evs); j++ {
			e := &evs[j]
			if e.Kind != events.DelayedRegister || e.Counter != "__kwEncoreSacrificeGroup" || len(e.IDs) != 0 {
				continue
			}
			o := g.Obj(e.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			name := o.Face().Name
			if name != "Tasha Yar, Out of Time" && name != "Blastoneuron" {
				continue
			}
			for _, id := range g.Zone(state.ZBattlefield, e.Player) {
				if h := g.Obj(id); h != nil && h.Face() != nil && h.Face().Name == "Holo-Engineer" {
					under[name]++
					break
				}
			}
		}
		evAt = len(evs)
		return nil
	}
	for seed := uint64(1); seed <= 120; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fixture-a", "fixture-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		evAt = 0
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v\nrecent %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions, res.RecentSources(m, 6))
		}
	}
	t.Logf("empty sacrifice groups under a Holo-Engineer: %v", under)
	for _, name := range []string{"Tasha Yar, Out of Time", "Blastoneuron"} {
		if under[name] < 3 {
			t.Fatalf("%s registered %d empty sacrifice groups under a Holo-Engineer; the fixture does not reach the rewritten copy", name, under[name])
		}
	}
}
