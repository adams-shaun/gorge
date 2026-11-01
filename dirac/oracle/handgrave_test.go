package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed decks over the generated
// handgrave pool, whose carriers pick over Origin$ Hand,Graveyard (the
// fetching player's own hand, then graveyard, one ask; gorge effSearchLibrary):
//
//   - Kastral, the Windcrested: a Bird card enters with a FINALITY counter
//     (Optional$ You, the confirm is posed first);
//   - Worldsoul's Rage: up to X lands enter tapped;
//   - Bonny Pall, Clearcutter: one land enters.
//
// Mill (Mind Sculpt), discard (Mind Rot), burn and Bird creatures put lands and
// Birds in both zones.

var khgDecks = [2][]cardset.Entry{
	{
		{Name: "Kastral, the Windcrested", Count: 3}, {Name: "Keeneye Aven", Count: 3}, {Name: "Bird Maiden", Count: 3},
		{Name: "Windcaller Aven", Count: 2}, {Name: "Mind Sculpt", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Shock", Count: 3}, {Name: "Mind Rot", Count: 3},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 7}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Bonny Pall, Clearcutter", Count: 3}, {Name: "Worldsoul's Rage", Count: 3}, {Name: "Bird Maiden", Count: 3},
		{Name: "Keeneye Aven", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Mind Sculpt", Count: 3},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 5},
	},
}

type khgCounts map[string]int

// khgPolicy counts, from gorge's pending decision, the carriers' picks.
func khgPolicy(seed uint64, n khgCounts) Policy {
	inner := eagerPolicy(seed, map[string]int{})
	run := 0 // consecutive Worldsoul's Rage search picks
	return PolicyFunc(func(c *PickCtx) Canon {
		a := inner.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil {
			return a
		}
		src := ""
		if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil {
			src = ob.Face().Name
		}
		opt, ok := c.G.GorgeOption(a)
		if d.ResumeKind != "search" || src != "Worldsoul's Rage" {
			run = 0
		}
		switch {
		case d.ResumeKind == "search_confirm" && ok && opt.Kind == "no" && src == "Kastral, the Windcrested":
			n["kastral_declined"]++
		case d.ResumeKind == "search" && ok && opt.Kind == "search":
			ob := c.G.E.G.Obj(opt.Obj)
			if ob == nil {
				break
			}
			switch ob.Zone {
			case state.ZHand:
				n["from_hand"]++
			case state.ZGraveyard:
				n["from_grave"]++
			}
			switch src {
			case "Kastral, the Windcrested":
				n["kastral_finality"]++ // every Kastral pick enters with a FINALITY counter
			case "Worldsoul's Rage":
				if run++; run == 2 {
					n["rage_2plus"]++
				}
			case "Bonny Pall, Clearcutter":
				n["bonny_land"]++
			}
		}
		return a
	})
}

// khgRun plays seeds 1..maxSeed; it returns the counts, the games played, the
// first divergence ("" if none) and the kernel stalls.
func khgRun(t *testing.T, maxSeed uint64) (n khgCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("handgrave")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range khgDecks {
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
	tbl, _, _, err := set.Matchup(khgDecks[0], khgDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	n = khgCounts{}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= maxSeed; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"khg-a", "khg-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, khgPolicy(seed, n), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v\nref %v\nker %v\n%v %s", seed, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions, d.Diff, d.Detail)
			return n, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return n, games, "", stalls
}

func TestHandgraveLockstep(t *testing.T) {
	t.Parallel()

	n, games, div, stalls := khgRun(t, 256)
	t.Logf("%d games; picks from hand %d, from graveyard %d; Kastral FINALITY entries %d, declined confirms %d; Worldsoul's Rage 2+ lands %d; Bonny Pall lands %d; stalls %v",
		games, n["from_hand"], n["from_grave"], n["kastral_finality"], n["kastral_declined"], n["rage_2plus"], n["bonny_land"], stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if n["from_hand"] < 10 || n["from_grave"] < 5 {
		t.Errorf("picks from hand %d, from graveyard %d, want at least 10 and 5", n["from_hand"], n["from_grave"])
	}
	if n["kastral_finality"] < 3 || n["kastral_declined"] < 3 {
		t.Errorf("Kastral FINALITY entries %d, declined confirms %d, want at least 3 each", n["kastral_finality"], n["kastral_declined"])
	}
	if n["rage_2plus"] < 3 {
		t.Errorf("Worldsoul's Rage resolutions moving 2+ lands %d, want at least 3", n["rage_2plus"])
	}
}

// TestHandgraveMutation: with the hand half of the pool dropped the lockstep
// must diverge.
func TestHandgraveMutation(t *testing.T) {
	dirac.DebugHandGraveNoHand = true
	defer func() { dirac.DebugHandGraveNoHand = false }()
	_, games, div, _ := khgRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugHandGraveNoHand", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}
