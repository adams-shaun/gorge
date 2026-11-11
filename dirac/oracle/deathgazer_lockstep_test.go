package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Deathgazer lockstep (CR 603.7, 506.4, 700.4): "Whenever Deathgazer
// blocks or becomes blocked by a nonblack creature, destroy that creature at
// end of combat" lowers to two AttackerBlockedByCreature halves, each
// registering an end-of-combat delayed destroy of the pair's other member.
// The kernel must destroy the same creatures at the same end of combat as
// gorge, and spare a black one (Burglar Rat).

var kdeathgazerDecks = [2][]cardset.Entry{
	{
		{Name: "Deathgazer", Count: 4}, {Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 5},
		{Name: "Burglar Rat", Count: 4}, {Name: "Shock", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Deathgazer", Count: 4}, {Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 5},
		{Name: "Burglar Rat", Count: 4}, {Name: "Shock", Count: 2},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
	},
}

type kdeathgazerStats struct {
	games, divs, stalls int
	blocks              int            // Deathgazer blocks a creature
	blocked             int            // a creature blocks an attacking Deathgazer
	nonblackBlocks      int            // ... of those, against a nonblack creature (the trigger's gate)
	byName              map[string]int // family creatures in a chosen block, either role
}

// kdeathgazerPolicy attacks and blocks whenever it can, preferring a block
// that pairs Deathgazer with a creature (either role), and casts or plays
// lands otherwise.
func kdeathgazerPolicy(seed uint64, st *kdeathgazerStats, family map[string]bool) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers || c.D.Kind == DBlockers {
			want := ActAttack
			if c.D.Kind == DBlockers {
				want = ActBlock
			}
			pick, other := -1, -1
			for i := range c.D.Options {
				o := &c.D.Options[i]
				if o.Kind() != want {
					continue
				}
				if c.G != nil && (family[gorgeOptionName(c, *o)] || want == ActBlock && family[gorgeAttackerName(c, *o)]) {
					if pick < 0 {
						pick = i
					}
				} else if other < 0 {
					other = i
				}
			}
			switch {
			case pick >= 0:
				if want == ActBlock {
					o := c.D.Options[pick]
					st.byName[gorgeOptionName(c, o)]++
					st.byName[gorgeAttackerName(c, o)]++
					if family[gorgeOptionName(c, o)] {
						st.blocks++
						if gorgeAttackerName(c, o) != "Burglar Rat" {
							st.nonblackBlocks++
						}
					} else {
						st.blocked++
						if gorgeOptionName(c, o) != "Burglar Rat" {
							st.nonblackBlocks++
						}
					}
				}
				return c.D.Options[pick]
			case other >= 0:
				return c.D.Options[other]
			}
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActPlayLand || o.Kind() == ActCast {
				return o
			}
		}
		return rnd.Pick(c)
	})
}

// runDeathgazer plays seeds 1..seeds of the constructed matchup.
func runDeathgazer(t *testing.T, seeds int, decksIn [2][]cardset.Entry, family map[string]bool) *kdeathgazerStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("deathgazer")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range decksIn {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	// Precondition: kgen lowers every card of the decks (a refused card does
	// not build a matchup table), so the lockstep reads the lowered triggers.
	tbl, _, _, err := set.Matchup(decksIn[0], decksIn[1])
	if err != nil {
		t.Fatalf("matchup table: %v", err)
	}
	st := &kdeathgazerStats{byName: map[string]int{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"deathgazer-a", "deathgazer-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kdeathgazerPolicy(seed, st, family), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return st
}

// TestDeathgazerLockstep plays Deathgazer through both engines, blocking and
// being blocked wherever the policy can.
func TestDeathgazerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDeathgazer(t, 128, kdeathgazerDecks, map[string]bool{"Deathgazer": true})
	t.Logf("%d games, %d divergences, %d stalls, Deathgazer blocks %d, blocked %d, nonblack %d",
		st.games, st.divs, st.stalls, st.blocks, st.blocked, st.nonblackBlocks)
	if st.blocks < 10 {
		t.Errorf("only %d Deathgazer blocks", st.blocks)
	}
	if st.blocked < 10 {
		t.Errorf("only %d blocks of an attacking Deathgazer", st.blocked)
	}
}

// The same two-halves shape over the family it shares with Deathgazer: the
// "blocks or becomes blocked by" cards whose end-of-combat delayed body
// destroys (Cockatrice, the Basilisks), deals 1 damage to
// (Tangle Asp's kin) or gains control of (Tolarian Entrancer) the other
// creature of the pair. They lower through the same delayed registration, so
// they play here in lockstep too.
var kdeathgazerFamilyDecks = [2][]cardset.Entry{
	{
		{Name: "Abomination", Count: 2}, {Name: "Cockatrice", Count: 2}, {Name: "Deathgazer", Count: 2},
		{Name: "Dread Specter", Count: 3}, {Name: "Infernal Medusa", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Swamp", Count: 12}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Rock Basilisk", Count: 2}, {Name: "Sawtooth Ogre", Count: 2}, {Name: "Tangle Asp", Count: 3},
		{Name: "Thicket Basilisk", Count: 2}, {Name: "Tolarian Entrancer", Count: 3}, {Name: "Venomous Dragonfly", Count: 2},
		{Name: "Deathgazer", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 3},
	},
}

var kdeathgazerFamily = map[string]bool{
	"Abomination": true, "Cockatrice": true, "Deathgazer": true, "Dread Specter": true,
	"Infernal Medusa": true, "Rock Basilisk": true, "Sawtooth Ogre": true, "Tangle Asp": true,
	"Thicket Basilisk": true, "Tolarian Entrancer": true, "Venomous Dragonfly": true,
}

// TestDeathgazerFamilyLockstep plays the family against each other, blocking
// and being blocked wherever the policy can.
func TestDeathgazerFamilyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runDeathgazer(t, 192, kdeathgazerFamilyDecks, kdeathgazerFamily)
	t.Logf("%d games, %d divergences, %d stalls, blocks %d, blocked %d",
		st.games, st.divs, st.stalls, st.blocks, st.blocked)
	if st.blocks < 10 {
		t.Errorf("only %d family blocks", st.blocks)
	}
	if st.blocked < 10 {
		t.Errorf("only %d blocks of an attacking family creature", st.blocked)
	}
	for name := range kdeathgazerFamily {
		if st.byName[name] == 0 {
			t.Errorf("%s never blocked or was blocked: its triggers were not played", name)
		}
	}
}
