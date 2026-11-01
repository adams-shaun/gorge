package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The three- and four-hybrid-pip lockstep (CR 107.4e, 601.2b):
// constructed matchups over the generated hybridn pool, every printed cost
// with three hybrid pips ({B/R}{B/R}{B/R}, {2}{R/W}{R/W}{R/W}) or four
// ({R/W}x4) or four different pairs (Leyline of the Guildpact,
// {G/W}{G/U}{B/G}{R/G}, beside three older {R/W}x4 cards). Leyline's
// AllBasicLandType static makes every land its controller has every basic
// land type, so each taps for any colour (CR 305.6). gorge announces each pip
// one by one, each menu gated on the whole cost's feasibility, and its
// potential planner resolves the pips with pip 0 the fastest varying.
//
// As in hybord_test.go, the autopay surface offers no hybrid cast (gorge's
// cast planner excludes hybrid costs); the autopay seeds still run and must
// agree on every decision.
var khybridnDecks = [2][]cardset.Entry{
	{
		{Name: "Eclipsed Boggart", Count: 4},         // {B/R}x3
		{Name: "Eclipsed Elf", Count: 4},             // {B/G}x3
		{Name: "Reaping Willow", Count: 3},           // {1}{W/B}x3
		{Name: "Fireborn Knight", Count: 4},          // {R/W}x4
		{Name: "Jet, Freedom Fighter", Count: 1},     // {2}{R/W}x3
		{Name: "Leyline of the Guildpact", Count: 2}, // {G/W}{G/U}{B/G}{R/G}
		{Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 5}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Eclipsed Merrow", Count: 5},        // {W/U}x3
		{Name: "Voracious Tome-Skimmer", Count: 5}, // {U/B}x3
		{Name: "Glister Bairn", Count: 4},          // {2}{G/U}x3
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
		{Name: "Plains", Count: 6}, {Name: "Forest", Count: 6},
	},
}

var khybridnFour = []string{"Fireborn Knight", "Resolute Rider", "Thunderous Snapper", "Leyline of the Guildpact"}

var khybridnThree = []string{
	"Eclipsed Boggart", "Eclipsed Elf", "Reaping Willow", "Jet, Freedom Fighter",
	"Eclipsed Merrow", "Voracious Tome-Skimmer", "Glister Bairn",
}

type khybridnTally struct {
	hybOrderTally
	pipIdx      int // the pip ask's index within its cast's run of pip asks
	deepAsks    int // pip asks at index >= 2
	gatedAsks   int // pip asks that offered exactly one colour
	surfaceAsks [2]int
}

func khybridnPolicy(seed uint64, tally *khybridnTally) Policy {
	rnd := RandomPolicy(seed)
	payCode, _ := chooseCode("pay")
	pay := ActChoose + CanonKind(payCode)
	return PolicyFunc(func(c *PickCtx) Canon {
		var buf []Canon
		isPip := len(c.D.Options) > 0
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate, ActPlayLand:
				buf = append(buf, a)
			}
			if a.Kind() != pay {
				isPip = false
			}
		}
		var pick Canon
		if len(buf) == 0 {
			pick = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			pick = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if !isPip {
			tally.pipIdx = 0
			return pick
		}
		tally.pipAsks++
		if tally.pipIdx >= 2 {
			tally.deepAsks++
		}
		tally.pipIdx++
		if len(c.D.Options) == 1 {
			tally.gatedAsks++
		}
		return pick
	})
}

func khybridnSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("hybridn")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range khybridnDecks {
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
			t.Fatalf("deck %d: %d cards, want 40", s, len(decks[s]))
		}
	}
	return reg, set, decks
}

// khybridnRun plays seeds lo..hi and returns the first divergence or stall
// ("" when none) with the tally.
func khybridnRun(t *testing.T, lo, hi uint64) (string, khybridnTally) {
	t.Helper()
	reg, set, decks := khybridnSetup(t)
	tbl, _, _, err := set.Matchup(khybridnDecks[0], khybridnDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	total := khybridnTally{hybOrderTally: hybOrderTally{casts: map[string]int{}}}
	var tally khybridnTally
	for seed := lo; seed <= hi; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		tally = khybridnTally{}
		tally.reset()
		m, err := NewMatchup(reg, [2]string{"hybn-a", "hybn-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, khybridnPolicy(seed, &tally), RunConfig{
			Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tally.check,
		})
		CloseSide(k)
		if res.Div != nil {
			lo := max(0, res.Div.At-8)
			return fmt.Sprintf("seed %d: %s %v\nlast actions %v", seed, res.Div, res.Div.Diff, actionStrings(res.Actions[lo:res.Div.At])), total
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) { // a game that reaches MaxTurns ends "turns"
			return fmt.Sprintf("seed %d stalled: %s", seed, res.Stall), total
		}
		total.pipAsks += tally.pipAsks
		total.deepAsks += tally.deepAsks
		total.gatedAsks += tally.gatedAsks
		for n, c := range tally.casts {
			total.casts[n] += c
		}
	}
	return "", total
}

func TestHybridnLockstep(t *testing.T) {
	t.Parallel()

	fail, tot := khybridnRun(t, 1, 256)
	if fail != "" {
		t.Fatal(fail)
	}
	three := 0
	for _, n := range khybridnThree {
		three += tot.casts[n]
	}
	four := 0
	for _, n := range khybridnFour {
		four += tot.casts[n]
	}
	t.Logf("256 seeds: %d pip asks (%d at index >= 2, %d offering one colour); 3-pip casts %d, 4-pip casts %d; casts %v",
		tot.pipAsks, tot.deepAsks, tot.gatedAsks, three, four, tot.casts)
	if three < 20 {
		t.Errorf("3-pip casts = %d, want at least 20", three)
	}
	if four < 5 {
		t.Errorf("4-pip casts = %d, want at least 5", four)
	}
	if tot.deepAsks < 50 {
		t.Errorf("pip asks at index >= 2 = %d, want at least 50", tot.deepAsks)
	}
	if tot.gatedAsks < 10 {
		t.Errorf("pip asks offering one colour = %d, want at least 10", tot.gatedAsks)
	}
}

// TestHybridnMutation: each mutation of the pip walks must make the
// lockstep diverge within 64 seeds. It is not parallel: it flips package
// variables, and parallel tests resume only after every serial test has
// finished.
//   - DebugHybridNPipGate: the pip asks at index >= 2 offer every colour
//     (the whole-cost feasibility gate ignores the later pips).
//
// The witness-order mutation (dirac.DebugHybridNPipOrder) does not diverge
// in this pool: every pip of a printed cost is announced before payment, so
// the planner witness never resolves a 3-pip cost here. It is pinned by
// dirac.TestHybridNWitnessOrder instead.
func TestHybridnMutation(t *testing.T) {
	for _, m := range []struct {
		name string
		flag *bool
	}{
		{"pip gate", &dirac.DebugHybridNPipGate},
	} {
		*m.flag = true
		fail, _ := khybridnRun(t, 1, 64)
		*m.flag = false
		if fail == "" {
			t.Errorf("%s: the mutation did not diverge within 64 seeds", m.name)
			continue
		}
		if strings.Contains(fail, "stalled") {
			t.Errorf("%s: the mutation stalled instead of diverging: %s", m.name, fail)
			continue
		}
		t.Logf("%s: diverged as expected: %.160s", m.name, fail)
	}
}
