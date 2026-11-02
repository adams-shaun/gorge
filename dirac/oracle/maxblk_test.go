package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The MinMaxBlocker Max$ 1 lockstep: "can't be blocked by
// more than one creature" on the Card.Self carriers, an equipment, an aura
// and the power / counter scoped lords, beside menace attackers (a floor
// above the cap leaves the attacker never offered) and a deck of many small
// creatures that can double block. The kernel's per-pick cap must offer the
// pairs gorge's KBlockers does at every pick of a declaration.

var kmaxblkDecks = [][2][]cardset.Entry{{
	{
		{Name: "Safewright Cavalry", Count: 4}, {Name: "Professional Wrestler", Count: 4}, {Name: "Rope", Count: 3},
		{Name: "Flopsie, Bumi's Buddy", Count: 2}, {Name: "Michelangelo, Mutant BFF", Count: 2},
		{Name: "Meltstrider's Resolve", Count: 2}, {Name: "Viashino Runner", Count: 3}, {Name: "Boggart Brute", Count: 3},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 1}, {Name: "Swamp", Count: 1},
		{Name: "Island", Count: 1},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Walking Corpse", Count: 6}, {Name: "Hill Giant", Count: 5},
		{Name: "Gray Ogre", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 5},
	},
}}

// kmaxblkCapped names the Card.Self carriers: their attacks are always capped.
var kmaxblkCapped = map[string]bool{"Safewright Cavalry": true, "Professional Wrestler": true}

type kmaxblkStats struct {
	games, divs, stalls int
	cappedBlocks        int // block picks against a Card.Self max-1 attacker
	cappedMulti         int // ... with two or more distinct blockers offered for it
}

// kmaxblkPolicy attacks and blocks whenever it can (so the games reach the
// declarations), else random; it counts the picks against capped attackers.
func kmaxblkPolicy(seed uint64, st *kmaxblkStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActAttack || k == ActBlock {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActBlock && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Attacker); ob != nil && ob.Face() != nil && kmaxblkCapped[ob.Face().Name] {
					st.cappedBlocks++
					blockers := map[any]bool{}
					for _, x := range c.D.Options {
						if x.Kind() != ActBlock {
							continue
						}
						if xo, ok := c.G.GorgeOption(x); ok && xo.Attacker == o.Attacker {
							blockers[xo.Obj] = true
						}
					}
					if len(blockers) >= 2 {
						st.cappedMulti++
					}
				}
			}
		}
		return a
	})
}

// runMaxblk plays the constructed matchup over seeds 1..seeds; off sets the
// DebugMaxBlkOff mutation.
func runMaxblk(t *testing.T, seeds int, off bool) kmaxblkStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("maxblk")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugMaxBlkOff
	dirac.DebugMaxBlkOff = off
	defer func() { dirac.DebugMaxBlkOff = old }()
	var st kmaxblkStats
	for mi, md := range kmaxblkDecks {
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
			m, err := NewMatchup(reg, [2]string{"kmaxblk-a", "kmaxblk-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kmaxblkPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestMaxblkLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runMaxblk(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d block picks against a max-1 attacker, %d with two or more blockers offered", st.games, st.divs, st.stalls, st.cappedBlocks, st.cappedMulti)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.cappedBlocks < 50 {
		t.Errorf("only %d block picks against a max-1 attacker, want >= 50", st.cappedBlocks)
	}
	if st.cappedMulti < 20 {
		t.Errorf("only %d of them with two or more blockers offered, want >= 20", st.cappedMulti)
	}
}

// TestMaxblkMutation turns the cap off: the kernel then offers a second
// blocker on a max-1 attacker that gorge does not, so the games must
// diverge. A green mutation means the lockstep never reaches a double block.
func TestMaxblkMutation(t *testing.T) {
	st := runMaxblk(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left the cap off in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
