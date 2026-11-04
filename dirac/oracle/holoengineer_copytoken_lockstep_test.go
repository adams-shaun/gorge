package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Replicator's token copy under Holo-Engineer's CreateToken rewriter: the copy
// mint is a token creation under the rewriter's controller, so the copy is
// replaced by a plain, untapped 3/3 Illusion (not a copy, not tapped: the
// replaced mint keeps neither the effect's TokenTapped$ nor its copy status).
// Replicator copying an Illusion token is the case where the rewriter's card
// equals the copied card, so only the rewriter's applying tells it apart.
func TestHoloEngineerCopyTokenLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	a := []cardset.Entry{
		{Name: "Holo-Engineer", Count: 1}, {Name: "Replicator", Count: 6},
		{Name: "Gwyndala, the Progeny", Count: 4}, {Name: "Tricorder", Count: 3},
		{Name: "Type-2 Phaser", Count: 3}, {Name: "Savvy Profiteer", Count: 3},
		{Name: "Forest", Count: 10}, {Name: "Island", Count: 10},
	}
	b := []cardset.Entry{
		{Name: "Holo-Engineer", Count: 1}, {Name: "Replicator", Count: 5},
		{Name: "Gwyndala, the Progeny", Count: 5}, {Name: "Academy Instructor", Count: 4},
		{Name: "Savvy Profiteer", Count: 4}, {Name: "Bat'leth", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Island", Count: 9},
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
	copies := 0 // Replicator activations by a player controlling a Holo-Engineer
	evAt := 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		g, evs := ref.E.G, ref.E.L.Events
		if len(evs) < evAt {
			evAt = 0
		}
		for j := evAt; j < len(evs); j++ {
			e := &evs[j]
			if e.Kind != events.AbilityPush {
				continue
			}
			if o := g.Obj(e.Obj); o == nil || o.Face() == nil || o.Face().Name != "Replicator" {
				continue
			}
			for _, id := range g.Zone(state.ZBattlefield, e.Player) {
				if h := g.Obj(id); h != nil && h.Face() != nil && h.Face().Name == "Holo-Engineer" {
					copies++
					break
				}
			}
		}
		evAt = len(evs)
		return nil
	}
	for seed := uint64(1); seed <= 60; seed++ {
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
	t.Logf("%d Replicator activations under a Holo-Engineer", copies)
	if copies < 5 {
		t.Fatalf("only %d Replicator activations under a Holo-Engineer; the fixture does not reach the rewritten copy", copies)
	}
}
