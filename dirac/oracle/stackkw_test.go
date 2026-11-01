package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The AffectedZone$ Stack keyword-grant lockstep: Sami and
// Witherbloom grant affinity (a ReduceCost the kernel prices through its
// ordinary cost walk) and Quandrix grants cascade to the instants and
// sorceries its controller casts from hand (the kernel's StatStackKw, read
// live when the cast trigger queues, as gorge's cascadeInstances is). Deck A
// is U/R/W artifacts (Sami, artifacts to count, instants, Bloodbraid Elf's
// printed cascade beside the grant in deck B); deck B the SOS colleges
// (Witherbloom, Quandrix, instants and sorceries, creatures to count).
//
// Improvise (Ironheart, Inspiring Statuary) and casualty (Silverquill) grants
// are proved by TestStackkwGrantLockstep (stackkw_grant_test.go): gorge
// offers a tap-paid or casualty cast only against a floating pool, which the
// canonical vocabulary builds only through an ETB mana trigger (Iridescent
// Tiger, Burning-Tree Emissary), so that test's decks carry them.

var kstackkwDecks = [2][2][]cardset.Entry{{
	{
		{Name: "Sami, Wildcat Captain", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Sol Ring", Count: 3},
		{Name: "Bonesplitter", Count: 3}, {Name: "Pilgrim's Eye", Count: 2},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Impulse", Count: 3},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Witherbloom, the Balancer", Count: 3}, {Name: "Quandrix, the Proof", Count: 3},
		{Name: "Opt", Count: 2}, {Name: "Impulse", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Brainstorm", Count: 2},
		{Name: "Bloodbraid Elf", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Walking Corpse", Count: 2}, {Name: "Giant Spider", Count: 1},
		{Name: "Island", Count: 5}, {Name: "Forest", Count: 4}, {Name: "Swamp", Count: 3}, {Name: "Plains", Count: 1}, {Name: "Mountain", Count: 2},
	},
}, {
	{
		{Name: "Witherbloom, the Balancer", Count: 3}, {Name: "Quandrix, the Proof", Count: 3},
		{Name: "Opt", Count: 2}, {Name: "Impulse", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Brainstorm", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Walking Corpse", Count: 3}, {Name: "Giant Spider", Count: 1},
		{Name: "Island", Count: 5}, {Name: "Forest", Count: 5}, {Name: "Swamp", Count: 4}, {Name: "Plains", Count: 1},
	},
	{
		{Name: "Sami, Wildcat Captain", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Sol Ring", Count: 3},
		{Name: "Bonesplitter", Count: 3}, {Name: "Pilgrim's Eye", Count: 2},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Impulse", Count: 3},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 3},
	},
}}

// kstackkwGeneric is the printed generic part of the spells an affinity
// grant can reduce.
var kstackkwGeneric = map[string]int{
	"Divination": 2, "Impulse": 1, "Mind Stone": 1, "Sol Ring": 1, "Bonesplitter": 1, "Pilgrim's Eye": 3,
	"Sami, Wildcat Captain": 4, "Quandrix, the Proof": 4, "Bloodbraid Elf": 2, "Giant Spider": 4, "Walking Corpse": 1,
	"Witherbloom, the Balancer": 6,
}

var kstackkwSpells = map[string]bool{"Shock": true, "Lightning Bolt": true, "Opt": true, "Impulse": true, "Divination": true, "Brainstorm": true}

// kstackkwPolicy plays the heuristic bot (a seeded random one every fourth
// seed) and counts the casts that exercise a grant: an affinity-reduced cast
// (a spell with a generic part under a granter whose count is above zero) and
// an instant or sorcery cast from hand while a Quandrix is out (a granted
// cascade).
func kstackkwPolicy(seed uint64, cnt map[string]int) Policy {
	base := Policy(HeuristicPolicy(seed))
	if seed%4 == 0 {
		base = RandomPolicy(seed)
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		a := base.Pick(c)
		if c.G == nil || a.Kind() != ActCast {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok {
			return a
		}
		ob := c.G.E.G.Obj(o.Obj)
		if ob == nil || ob.Face() == nil {
			return a
		}
		name := ob.Face().Name
		who := state.PlayerID(c.D.Player)
		has := map[string]bool{}
		creatures, arts := 0, 0
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, who) {
			if bo := c.G.E.G.Obj(id); bo != nil && bo.Face() != nil {
				has[bo.Face().Name] = true
				if bo.EffectiveIsCreature() {
					creatures++
				}
				if c.G.E.IsArtifact(id) {
					arts++
				}
			}
		}
		if has["Quandrix, the Proof"] && kstackkwSpells[name] {
			cnt["cascade"]++
		}
		if kstackkwGeneric[name] > 0 && (has["Sami, Wildcat Captain"] && arts > 0 || has["Witherbloom, the Balancer"] && kstackkwSpells[name] && creatures > 0) {
			cnt["affinity"]++
		}
		return a
	})
}

// runStackkw plays seeds 1..seeds, alternating the two matchups, and
// returns the games, the divergences, the capacity stalls and the counts.
func runStackkw(t *testing.T, seeds int, report bool) (games, divs, stalls int, cnt map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stackkw")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][2][]*cards.Card
	var tbls [2]*kop.Table
	for mi, md := range kstackkwDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	cnt = map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % 2
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kstackkw-a", "kstackkw-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kstackkwPolicy(seed, cnt), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if !report || divs >= 3 {
				break
			}
		}
	}
	return games, divs, stalls, cnt
}

func TestStackkwLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, cnt := runStackkw(t, 256, true)
	t.Logf("%d games, %d divergences, %d stalls, granted plays %v", games, divs, stalls, cnt)
	for _, w := range []string{"affinity", "cascade"} {
		if cnt[w] < 20 {
			t.Errorf("only %d %s plays (want >= 20)", cnt[w], w)
		}
	}
}

// TestStackkwMutation makes the kernel ignore the keyword grants (an
// affinity granted from the battlefield still applies through its ordinary
// ReduceCost): the granted cascade vanishes, so the lockstep must diverge.
func TestStackkwMutation(t *testing.T) {
	old := dirac.DebugStackKwOff
	dirac.DebugStackKwOff = true
	defer func() { dirac.DebugStackKwOff = old }()
	games, divs, _, _ := runStackkw(t, 64, false)
	if divs == 0 {
		t.Fatalf("grants ignored in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
