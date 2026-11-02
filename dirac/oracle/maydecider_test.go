package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The OptionalDecider$ lockstep: a Draw's "you may draw"
// (Starwinder's combat-damage trigger, Cactarantula's becomes-target
// trigger, Consecrated Sphinx on the opponent's draws, Vex's "that spell's
// controller may draw") and a DealDamage's "that creature's controller may
// have it deal 1 damage" (Requiem Monolith) are asked yes / no of the named
// decider (gorge draw_optional, dealdamage_optional). Requiem Monolith and
// Shock make Cactarantula a target; Vex counters the other deck's spells.

var kmaydeciderDecks = [2][]cardset.Entry{
	{
		{Name: "Requiem Monolith", Count: 4}, {Name: "Vex", Count: 3}, {Name: "Consecrated Sphinx", Count: 2},
		{Name: "Starwinder", Count: 3}, {Name: "Walking Corpse", Count: 6},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 9}, {Name: "Lonely Arroyo", Count: 4},
	},
	{
		{Name: "Cactarantula", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 6}, {Name: "Lonely Arroyo", Count: 4},
	},
}

// kmaydeciderStats counts what the policy saw gorge pose.
type kmaydeciderStats struct {
	draw      map[string]int // answers to a draw_optional ask, by option kind
	dmg       map[string]int // answers to a dealdamage_optional ask, by option kind
	dmgNonAct int            // Requiem damage asks posed to the non-active player
	star      int            // Starwinder draw asks
}

// kmaydeciderPolicy is the random policy that tallies the OptionalDecider$
// answers from gorge's pending decision.
func kmaydeciderPolicy(seed uint64, st *kmaydeciderStats) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok {
			return a
		}
		switch d.ResumeKind {
		case "draw_optional":
			st.draw[o.Kind]++
			if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil && ob.Face().Name == "Starwinder" {
				st.star++
			}
		case "dealdamage_optional":
			st.dmg[o.Kind]++
			if d.Player != c.G.E.G.Active {
				st.dmgNonAct++
			}
		}
		return a
	})
}

func runMaydecider(t *testing.T, seeds int, mutate bool) (games, divs int, st *kmaydeciderStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("maydecider")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugMayDeciderCtrl
	dirac.DebugMayDeciderCtrl = mutate
	defer func() { dirac.DebugMayDeciderCtrl = old }()
	st = &kmaydeciderStats{draw: map[string]int{}, dmg: map[string]int{}}
	var decks [2][]*cards.Card
	for s, d := range kmaydeciderDecks {
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
	tbl, _, _, err := set.Matchup(kmaydeciderDecks[0], kmaydeciderDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, kmaydeciderNames, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kmaydeciderPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) && !mutate {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
	}
	return games, divs, st
}

var kmaydeciderNames = [2]string{"kmaydecider-a", "kmaydecider-b"}

func TestMaydeciderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, st := runMaydecider(t, 256, false)
	t.Logf("%d games, %d divergences, draw asks %v (Starwinder %d), damage asks %v (non-active decider %d)", games, divs, st.draw, st.star, st.dmg, st.dmgNonAct)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if st.draw["yes"] == 0 || st.draw["no"] == 0 || st.draw["yes"]+st.draw["no"] < 20 {
		t.Errorf("draw asks %v: want both answers and at least 20", st.draw)
	}
	if st.dmgNonAct < 5 {
		t.Errorf("Requiem damage asks to the non-active player %d: want at least 5", st.dmgNonAct)
	}
	if st.star < 1 {
		t.Errorf("Starwinder draw asks %d: want at least 1", st.star)
	}
}

// TestMaydeciderMutation asks the resolving controller every
// OptionalDecider$ election (the named decider ignored): gorge asks the
// target's controller, so the decision streams must differ.
func TestMaydeciderMutation(t *testing.T) {
	games, divs, _ := runMaydecider(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation asked the controller in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
