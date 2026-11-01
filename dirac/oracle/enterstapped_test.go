package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The enterstapped lockstep (A): conditional enters-tapped lands, the
// ConditionPresent$ + ConditionCompare$ gate on a self ETB Tap replacement
// (gorge conditionMetBattlefield, which counts the battlefield for the
// entering permanent's controller EXCLUDING the entering object). The pool
// holds every sub-class A spelling the lowering reads, plus basics,
// planeswalkers, a legendary green creature, a Mount and a Vehicle so each
// condition goes both ways. Run as its own shard
// (-run '^TestConditionalEntersTappedLockstep$'), not inside the remainder Lockstep$ shard.
var enterstappedDecks = [][2][]cardset.Entry{{
	{
		{Name: "Blooming Marsh", Count: 4}, {Name: "Copperline Gorge", Count: 4},
		{Name: "Deathcap Glade", Count: 4}, {Name: "Haunted Ridge", Count: 4},
		{Name: "Ajani, Caller of the Pride", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Forest", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Realm of Koh", Count: 4}, {Name: "Kishla Village", Count: 4},
		{Name: "Cori Mountain Monastery", Count: 4}, {Name: "Argoth, Sanctum of Nature", Count: 4},
		{Name: "Kaseto, Orochi Archmage", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Plains", Count: 3}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 2}, {Name: "Forest", Count: 4},
	},
}, {
	{
		{Name: "Dedicated Commons", Count: 4}, {Name: "Fatehold Annex", Count: 4},
		{Name: "Wild Roads", Count: 4}, {Name: "Country Roads", Count: 4},
		{Name: "Ajani, Caller of the Pride", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Dedicated Commons", Count: 4}, {Name: "Fatehold Annex", Count: 4},
		{Name: "Wild Roads", Count: 4}, {Name: "Country Roads", Count: 4},
		{Name: "Weatherlight", Count: 4}, {Name: "Brightfield Glider", Count: 4},
		{Name: "Hill Giant", Count: 4},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
	},
}}

// enterstappedCarrier maps a carrier land to the ConditionPresent spelling it
// carries. Each spelling is counted entering tapped and untapped.
var enterstappedCarrier = map[string]string{
	"Blooming Marsh":            "GT2",
	"Copperline Gorge":          "GT2",
	"Deathcap Glade":            "LT2",
	"Haunted Ridge":             "LT2",
	"Dedicated Commons":         "PW_EQ0",
	"Fatehold Annex":            "PW_EQ0",
	"Kishla Village":            "BB_EQ0",
	"Cori Mountain Monastery":   "BB_EQ0",
	"Realm of Koh":              "BASIC_EQ0",
	"Argoth, Sanctum of Nature": "LEGEND_EQ0",
	"Wild Roads":                "MOUNT_EQ0",
	"Country Roads":             "MOUNT_EQ0",
}

// enterstappedPolicy plays a land whenever it can, then casts or activates, else
// random. Every decision it scans the gorge battlefield and counts each
// carrier's first entry as tapped or untapped (the branch the replacement
// took); the lockstep's projections compare the kernel's tapped state, so a
// wrong kernel branch diverges.
func enterstappedPolicy(seed uint64, counts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	seen := map[state.ObjID]bool{}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for _, ob := range c.G.E.G.Objs {
				if ob.Face() == nil || ob.Zone != state.ZBattlefield || seen[ob.ID] {
					continue
				}
				seen[ob.ID] = true
				sp, ok := enterstappedCarrier[ob.Face().Name]
				if !ok {
					continue
				}
				if ob.Tapped {
					counts[sp+"/tapped"]++
				} else {
					counts[sp+"/untapped"]++
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
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

func enterstappedEnv(t *testing.T) (cardset.Set, func(mi int) (*Matchup, error)) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("enterstapped")
	if err != nil {
		t.Fatal(err)
	}
	build := func(mi int) (*Matchup, error) {
		md := enterstappedDecks[mi]
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
		return NewMatchup(reg, [2]string{"kc10-a", "kc10-b"}, decks, 1)
	}
	return set, build
}

func TestConditionalEntersTappedLockstep(t *testing.T) {
	t.Parallel()

	set, build := enterstappedEnv(t)
	counts := map[string]int{}
	games := 0
	for mi := range enterstappedDecks {
		md := enterstappedDecks[mi]
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 1024; seed++ {
			m, err := build(mi)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, enterstappedPolicy(seed, counts), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier counts %v", games, counts)
	for _, sp := range []string{"GT2", "LT2", "PW_EQ0", "BB_EQ0", "BASIC_EQ0"} {
		for _, arm := range []string{"tapped", "untapped"} {
			if counts[sp+"/"+arm] < 10 {
				t.Errorf("%s entering %s: %d, want at least 10", sp, arm, counts[sp+"/"+arm])
			}
		}
	}
}

// TestConditionalEntersTappedMutation: each mutation of the lifted branch must diverge.
// (a) ignore the condition (always tapped); (b) count the entering land.
func TestConditionalEntersTappedMutation(t *testing.T) {
	set, build := enterstappedEnv(t)
	run := func(name string, mode uint8) {
		dirac.SetDebugEntersTappedMutate(mode)
		defer dirac.SetDebugEntersTappedMutate(0)
		diverged := false
		for mi := range enterstappedDecks {
			md := enterstappedDecks[mi]
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= 256 && !diverged; seed++ {
				m, err := build(mi)
				if err != nil {
					t.Fatal(err)
				}
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, enterstappedPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
				CloseSide(k)
				if res.Div != nil {
					t.Logf("%s: matchup %d seed %d diverged at decision %d", name, mi, seed, res.Div.At)
					diverged = true
				}
			}
		}
		if !diverged {
			t.Errorf("%s: mutation diverged in no game", name)
		}
	}
	run("always tapped", 1)
	run("count self", 2)
}
