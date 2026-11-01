package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: a Continuous AddAllCreatureTypes$ static
// lowers to the override's AllCreatureTypes (layer 4, CR 205.3m / 702.73a),
// the same record an Animate writes. Two carriers exercise both override
// shapes:
//
//   - Stalactite Dagger: AffectedDefined$ Equipped, an attached override
//     beside its +1/+1 pump (the pump split);
//   - Undercover Skrull: AffectedDefined$ Self behind
//     IsPresent$ Creature.YouOwn | PresentCompare$ GE2 | PresentZone$
//     Graveyard, a condition that must reach BOTH the pump and the override.
//
// The pool's chosen-type anthem (Adaptive Automaton), type lords (Elvish
// Archdruid, Goblin Chieftain, Merfolk Sovereign, Imperious Perfect) and the
// Count$Valid Elf payoff (Elvish Archdruid's mana ability) read the types, so
// "every creature type" changes what they count. Removal, Stitcher's Supplier
// and Satyr Wayfinder fill graveyards so the Skrull gate flips.

var kallcreatDecks = [][2][]cardset.Entry{{
	// green all-creature-types / lords, against red-green removal and mill
	{
		{Name: "Stalactite Dagger", Count: 3}, {Name: "Undercover Skrull", Count: 3},
		{Name: "Elvish Archdruid", Count: 3}, {Name: "Imperious Perfect", Count: 3},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Adaptive Automaton", Count: 1},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 19},
	},
	{
		{Name: "Stalactite Dagger", Count: 3}, {Name: "Undercover Skrull", Count: 3},
		{Name: "Goblin Chieftain", Count: 3}, {Name: "Goblin Piker", Count: 4},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Satyr Wayfinder", Count: 3}, {Name: "Adaptive Automaton", Count: 1},
		{Name: "Forest", Count: 11}, {Name: "Mountain", Count: 6},
	},
}}

// kallcreatStats counts the decisions that could read the layer-4 grant: the
// Dagger attached to a creature, and a Skrull whose graveyard gate is true.
type kallcreatStats struct {
	dagger, skrull int
}

// kallcreatPolicy records the two carriers' states at each decision, then
// answers at random.
func kallcreatPolicy(seed uint64, st *kallcreatStats) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			g := c.G.E.G
			dagger, skrull := false, false
			for i := range g.Objs {
				o := &g.Objs[i]
				if o.Zone != state.ZBattlefield || o.Face() == nil {
					continue
				}
				switch o.Face().Name {
				case "Stalactite Dagger":
					if o.AttachedTo != 0 {
						dagger = true
					}
				case "Undercover Skrull":
					if kallcreatGraveCreatures(g, o.Controller) >= 2 {
						skrull = true
					}
				}
			}
			if dagger {
				st.dagger++
			}
			if skrull {
				st.skrull++
			}
		}
		return rnd.Pick(c)
	})
}

// kallcreatGraveCreatures counts the creature cards in p's graveyard.
func kallcreatGraveCreatures(g *state.Game, p state.PlayerID) int {
	n := 0
	for _, id := range g.Zone(state.ZGraveyard, p) {
		if ob := g.Obj(id); ob != nil && ob.Face() != nil {
			for _, t := range ob.Face().Types {
				if t == "Creature" {
					n++
					break
				}
			}
		}
	}
	return n
}

// kallcreatRun plays the constructed matchup and returns the games, the
// divergences and the stalls.
func kallcreatRun(t *testing.T, seeds int, st *kallcreatStats) (games, divs, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("allcreat")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range kallcreatDecks {
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
			m, err := NewMatchup(reg, [2]string{"kallcreat-a", "kallcreat-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, kallcreatPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Logf("matchup %d seed %d: stall %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				divs++
				t.Logf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls
}

// TestAllcreatLockstep: 256 games, 0 divergences and 0 stalls, with enough
// attached Daggers and true Skrull gates for the grant to be read.
func TestAllcreatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st kallcreatStats
	games, divs, stalls := kallcreatRun(t, 256, &st)
	t.Logf("%d games, %d divergences, %d stalls; %d decisions with the Dagger attached, %d with Skrull's condition true",
		games, divs, stalls, st.dagger, st.skrull)
	if divs != 0 || stalls != 0 {
		t.Errorf("%d divergences, %d stalls", divs, stalls)
	}
	if st.dagger < 20 {
		t.Errorf("only %d decisions with the Dagger attached, want at least 20", st.dagger)
	}
	if st.skrull < 10 {
		t.Errorf("only %d decisions with Skrull's condition true, want at least 10", st.skrull)
	}
}

// TestAllcreatMutation ignores every override's AllCreatureTypes (the
// mutation switch in dirac/static_allcreatures.go): an equipped creature
// that is not naturally an Elf stops counting as one, so the lord's pump
// disappears and the lockstep must diverge.
func TestAllcreatMutation(t *testing.T) {
	old := dirac.DebugAllCreatNoOverride
	dirac.DebugAllCreatNoOverride = true
	defer func() { dirac.DebugAllCreatNoOverride = old }()
	var st kallcreatStats
	games, divs, _ := kallcreatRun(t, 128, &st)
	if divs == 0 {
		t.Fatalf("mutation dropped the override's all-creature-types grant in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
