package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The AdventureCard filter word's lockstep: Edgewall
// Inn's graveyard return, Howling Galefang's haste while the owner has an
// adventurer in exile, Frantic Firebolt and Hearth Elemental's graveyard
// counts and Beluna Grandsquall's cost reduction, beside adventurers cast
// as their Adventure halves. The kernel's cold PredAdventureCard reads must
// agree with gorge's wordAdventureCard at every decision.

var kadventurecardDecks = [][2][]cardset.Entry{{
	// A: red adventures
	{
		{Name: "Frantic Firebolt", Count: 3}, {Name: "Hearth Elemental", Count: 3}, {Name: "Howling Galefang", Count: 4},
		{Name: "Edgewall Inn", Count: 4}, {Name: "Beanstalk Wurm", Count: 3}, {Name: "Bramble Familiar", Count: 3},
		{Name: "Bellowing Bruiser", Count: 3}, {Name: "Galvanic Giant", Count: 3}, {Name: "Mountain", Count: 8},
		{Name: "Forest", Count: 6},
	},
	// B: G/R with the cost reducer. Sentinel of Lost Lore is kept out: gorge
	// targets the battlefield for its Origin$ Exile modes (see the report's
	// gorge finding), where the kernel (and Forge) target exile.
	{
		{Name: "Beluna Grandsquall", Count: 2}, {Name: "Frolicking Familiar", Count: 2}, {Name: "Ferocious Werefox", Count: 2},
		{Name: "Beanstalk Wurm", Count: 2}, {Name: "Bramble Familiar", Count: 2}, {Name: "Frantic Firebolt", Count: 2},
		{Name: "Hearth Elemental", Count: 2}, {Name: "Howling Galefang", Count: 2}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Forest", Count: 7}, {Name: "Island", Count: 3},
	},
}, {
	// A against itself: the Inn and Galefang from both seats
	{
		{Name: "Frantic Firebolt", Count: 3}, {Name: "Hearth Elemental", Count: 3}, {Name: "Howling Galefang", Count: 4},
		{Name: "Edgewall Inn", Count: 4}, {Name: "Beanstalk Wurm", Count: 3}, {Name: "Bramble Familiar", Count: 3},
		{Name: "Bellowing Bruiser", Count: 3}, {Name: "Galvanic Giant", Count: 3}, {Name: "Mountain", Count: 8},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Frantic Firebolt", Count: 3}, {Name: "Hearth Elemental", Count: 3}, {Name: "Howling Galefang", Count: 4},
		{Name: "Edgewall Inn", Count: 4}, {Name: "Beanstalk Wurm", Count: 3}, {Name: "Bramble Familiar", Count: 3},
		{Name: "Bellowing Bruiser", Count: 3}, {Name: "Galvanic Giant", Count: 3}, {Name: "Mountain", Count: 8},
		{Name: "Forest", Count: 6},
	},
}}

// kadventurecardStats is what the policy counts across the games.
type kadventurecardStats struct {
	innActs      int // Edgewall Inn activations posed
	galefangTurn int // (game, turn)s with a Galefang out and an adventurer exiled under its controller
}

// kadventurecardPolicy prefers casting or activating (so the Adventure half
// is cast when offered and the carriers are reached), else random, and
// counts the carrier states the gorge side poses.
func kadventurecardPolicy(seed uint64, st *kadventurecardStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	lastTurn := int32(-1)
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
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
		if c.G == nil {
			return a
		}
		g := c.G.E.G
		if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActActivate {
			if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Edgewall Inn" {
				st.innActs++
			}
		}
		if t := int32(g.Turn); t != lastTurn {
			lastTurn = t
			for p := state.PlayerID(0); p < 2; p++ {
				fang, onAdv := false, false
				for _, id := range g.Zone(state.ZBattlefield, p) {
					if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Howling Galefang" {
						fang = true
					}
				}
				for _, id := range g.Zone(state.ZExile, p) {
					if ob := g.Obj(id); ob != nil && ob.Card != nil && ob.Card.AlternateMode == "Adventure" {
						onAdv = true
					}
				}
				if fang && onAdv {
					st.galefangTurn++
				}
			}
		}
		return a
	})
}

// runAdventurecard plays the constructed matchups and returns the games,
// the divergences, the stalls and the carrier counts. mutate makes the
// kernel's reader answer false (the mutation must diverge).
func runAdventurecard(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kadventurecardStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("adventurecard")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugAdventureCardNever
	dirac.DebugAdventureCardNever = mutate
	defer func() { dirac.DebugAdventureCardNever = old }()
	for mi, md := range kadventurecardDecks {
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
			m, err := NewMatchup(reg, [2]string{"kadventurecard-a", "kadventurecard-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kadventurecardPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestAdventurecardLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, st := runAdventurecard(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, Edgewall Inn activations %d, Galefang turns with an adventurer exiled %d", games, divs, stalls, st.innActs, st.galefangTurn)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if st.galefangTurn < 20 {
		t.Errorf("only %d Howling Galefang turns with an adventurer in exile (want >= 20)", st.galefangTurn)
	}
	if st.innActs < 10 {
		t.Errorf("only %d Edgewall Inn activations (want >= 10)", st.innActs)
	}
}

// TestAdventurecardMutation makes the kernel's AdventureCard reader answer
// false: gorge's word is true for an adventurer in every zone, so the games
// must diverge. A green mutation means the lockstep never reaches a decision
// the word shapes.
func TestAdventurecardMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runAdventurecard(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation answered false in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
