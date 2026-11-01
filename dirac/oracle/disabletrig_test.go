package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The DisableTriggers enters lockstep (CR 603.2g): Torpor
// Orb (creatures), Karn, Argent Defender and Doorkeeper Thrull (artifacts
// and creatures) stop every trigger off a matching permanent entering, beside
// self-ETB creatures, "another creature enters" listeners, landfall, an
// equipment with an enters trigger, a token maker and a lifegain land (lands
// are not matched and must still trigger). The kernel's one gate in
// fireObjGen must drop exactly the placements gorge's per-line
// disableTriggersExcludes does.

var kdisabletrigDecks = [][2][]cardset.Entry{{
	{
		{Name: "Torpor Orb", Count: 2}, {Name: "Karn, Argent Defender", Count: 2}, {Name: "Doorkeeper Thrull", Count: 3},
		{Name: "Helpful Hunter", Count: 3}, {Name: "Dazzling Angel", Count: 2}, {Name: "Prideful Parent", Count: 3},
		{Name: "Good-Fortune Unicorn", Count: 2}, {Name: "Celestial Armor", Count: 1}, {Name: "Adventuring Gear", Count: 2},
		{Name: "Banishing Light", Count: 1}, {Name: "Reclamation Sage", Count: 2}, {Name: "Elfsworn Giant", Count: 1},
		{Name: "Blossoming Sands", Count: 3}, {Name: "Plains", Count: 7}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Torpor Orb", Count: 2}, {Name: "Karn, Argent Defender", Count: 1}, {Name: "Doorkeeper Thrull", Count: 2},
		{Name: "Helpful Hunter", Count: 3}, {Name: "Resolute Reinforcements", Count: 3}, {Name: "Prideful Parent", Count: 2},
		{Name: "Beast-Kin Ranger", Count: 3}, {Name: "Mossborn Hydra", Count: 2}, {Name: "Adventuring Gear", Count: 2},
		{Name: "Banishing Light", Count: 2}, {Name: "Reclamation Sage", Count: 2}, {Name: "Celestial Armor", Count: 1},
		{Name: "Blossoming Sands", Count: 3}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 6},
	},
}}

type kdisabletrigStats struct {
	games, divs, stalls int
	suppressed          int // matching permanents entering while a carrier is live
	suppressedETB       int // ... that print an enters trigger
	unsuppressed        int // enters trigger lines placed while a carrier is live (a land, or an artifact under Orb alone)
}

// kdisabletrigPolicy prefers plays and casts (so the games fill the board),
// else random; it counts the entering permanents against the live carriers.
func kdisabletrigPolicy(seed uint64, st *kdisabletrigStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	seen := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			var fresh []*state.Object
			orb, wide := false, false // Orb alone covers creatures; Karn / Thrull artifacts too
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.Zone(state.ZBattlefield, p) {
					ob := g.Obj(id)
					if ob == nil || ob.Face() == nil {
						continue
					}
					switch ob.Face().Name {
					case "Torpor Orb":
						orb = true
					case "Karn, Argent Defender", "Doorkeeper Thrull":
						wide = true
					}
					if !seen[id] {
						seen[id] = true
						fresh = append(fresh, ob)
					}
				}
			}
			if orb || wide {
				for _, ob := range fresh {
					var creature, artifact, land bool
					for _, ty := range ob.Face().Types {
						switch ty {
						case "Creature":
							creature = true
						case "Artifact":
							artifact = true
						case "Land":
							land = true
						}
					}
					name := ob.Face().Name
					hasETB := name == "Helpful Hunter" || name == "Dazzling Angel" || name == "Prideful Parent" || name == "Reclamation Sage" ||
						name == "Resolute Reinforcements" || name == "Elfsworn Giant"
					switch {
					case creature || artifact && wide:
						st.suppressed++
						if hasETB {
							st.suppressedETB++
						}
					case artifact && name == "Celestial Armor", land && name == "Blossoming Sands":
						st.unsuppressed++
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActPlayLand || k == ActCast {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runDisabletrig plays the constructed matchup over seeds 1..seeds; off sets
// the DebugDisETBOff mutation.
func runDisabletrig(t *testing.T, seeds int, off bool) kdisabletrigStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("disabletrig")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugDisETBOff
	dirac.DebugDisETBOff = off
	defer func() { dirac.DebugDisETBOff = old }()
	var st kdisabletrigStats
	for mi, md := range kdisabletrigDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kdisabletrig-a", "kdisabletrig-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, kdisabletrigPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			st.games++
			if d := res.Div; d != nil {
				st.divs++
				if !off {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				st.stalls++
				if !off {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return st
}

func TestDisabletrigLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDisabletrig(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d suppressed enters (%d with an enters trigger), %d unsuppressed enters triggers placed with a carrier live", st.games, st.divs, st.stalls, st.suppressed, st.suppressedETB, st.unsuppressed)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.suppressed < 40 {
		t.Errorf("only %d suppressed enters, want >= 40", st.suppressed)
	}
	if st.unsuppressed < 20 {
		t.Errorf("only %d unsuppressed enters triggers with a carrier live, want >= 20", st.unsuppressed)
	}
}

// TestDisabletrigMutation turns the gate off: the kernel then queues the
// enters triggers gorge drops, so the games must diverge. A green mutation
// means the lockstep never reaches a suppressed trigger.
func TestDisabletrigMutation(t *testing.T) {
	st := runDisabletrig(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left the gate off in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
