package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestHBotParity plays FDN pairings in lockstep with gorge's heuristic
// autopay bot choosing and asks the native port (dirac/search/hbot) every
// decision in shadow (HBotParityGame): every compared decision agrees. A
// decision gorge answers off the canonical surface (a cast whose cost's
// shape the planner declines, cast by hand) is not compared.
func TestHBotParity(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	type tally struct{ n, agree int }
	kinds := map[string]*tally{}
	ps := PairingsFrom(pool, 0, 12, 78)
	mk := func(tbl *kop.Table) KernelBot { return hbot.New(HBotFacts(c.Reg, tbl, nil)) }
	for _, p := range ps {
		res, err := HBotParityGame(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000, mk)
		if err != nil {
			t.Fatalf("pair %d: %v", p.Index, err)
		}
		if res.Res.Div != nil {
			t.Fatalf("pair %d diverged: %s", p.Index, res.Res.Div)
		}
		for _, r := range res.Recs {
			if r.Unoffered {
				continue
			}
			k := kinds[r.Kind]
			if k == nil {
				k = &tally{}
				kinds[r.Kind] = k
			}
			k.n++
			if r.Agree {
				k.agree++
			} else {
				t.Logf("pair %d: %s", p.Index, r.Where)
			}
		}
	}
	for _, k := range []string{"priority", "attackers", "blockers", "target"} {
		if x := kinds[k]; x == nil || x.n < 20 {
			t.Fatalf("%s: too few decisions compared: %+v", k, x)
		}
	}
	for k, x := range kinds {
		t.Logf("%-16s %5d %.4f", k, x.n, float64(x.agree)/float64(x.n))
		if x.agree != x.n {
			t.Errorf("%s: %d of %d decisions disagree", k, x.n-x.agree, x.n)
		}
	}
}

// TestHBotParityFDNPairs replays FDN pairings that once disagreed: the
// harness reading a trigger order past its batch into the next one (pair
// 24, two Clinquant Skymage draw triggers), and the port's hand taps toward
// a shape cast gorge's potential walk leaves out (Eaten Alive's {B} beside a
// Forest and Thornwood Falls floated a land pick, pair 71; Goblin
// Negotiation's {R}{R} on one Swiftwater Cliffs held Burst Lightning, pair
// 75); khbot's pairings of the FDN pool's eval split. Every compared
// decision agrees.
func TestHBotParityFDNPairs(t *testing.T) {
	t.Parallel()

	c, _ := testCorpus(t)
	pool, err := ReadPool(filepath.Join(PoolInputs("fdn"), "pool.tsv"), "eval")
	if err != nil {
		t.Skipf("no FDN deck pool (khbot's eval split): %v", err)
	}
	mk := func(tbl *kop.Table) KernelBot { return hbot.New(HBotFacts(c.Reg, tbl, nil)) }
	for _, i := range []int{24, 71, 75} {
		p := PairingsFrom(pool, i, 1, 78)[0]
		res, err := HBotParityGame(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000, mk)
		if err != nil {
			t.Fatalf("pair %d: %v", i, err)
		}
		if res.Res.Div != nil {
			t.Fatalf("pair %d diverged: %s", i, res.Res.Div)
		}
		for _, r := range res.Recs {
			if !r.Unoffered && !r.Agree {
				t.Errorf("pair %d: %s", i, r.Where)
			}
		}
	}
}

// TestHBotPlaysFDNGames plays the port against itself on the pool table
// with the default kernel configuration (every castable spell offered,
// a wider surface than gorge's): every answer is an option, every game
// ends, and no Pick allocates.
func TestHBotPlaysFDNGames(t *testing.T) {
	// serial: testing.AllocsPerRun panics in a parallel test

	c, pool := testCorpus(t)
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	dirac.Register(tbl)
	bot := hbot.New(HBotFacts(c.Reg, tbl, nil))
	var keepS []*dirac.State
	var keepD []dirac.Decision
	for _, p := range PairingsFrom(pool, 0, 16, 78) {
		m, _, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			t.Fatal(err)
		}
		var decks [2][]dirac.CardID
		for s := 0; s < 2; s++ {
			for _, gc := range m.Decks[s] {
				i, ok := set.Lookup(cardName(gc))
				if !ok {
					t.Fatalf("%s not in the pool", cardName(gc))
				}
				decks[s] = append(decks[s], set.PoolID(i))
			}
		}
		var s dirac.State
		s.Init(tbl, decks[0], decks[1], p.Seed, dirac.Config{})
		var d dirac.Decision
		n := 0
		for ; n < 20000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			i := bot.Pick(&s, &d)
			if i < 0 || i >= int(d.N) {
				t.Fatalf("pair %d decision %d: pick %d of %d", p.Index, n, i, d.N)
			}
			if d.N > 1 && n%3 == 0 && len(keepS) < 600 {
				c := new(dirac.State)
				c.CopyFrom(&s)
				keepS, keepD = append(keepS, c), append(keepD, d)
			}
			s.Apply(d.Options[i])
		}
		if over, _ := s.Terminal(); !over {
			t.Fatalf("pair %d: not over after %d decisions", p.Index, n)
		}
	}
	allocs := testing.AllocsPerRun(3, func() {
		for i := range keepS {
			bot.Pick(keepS[i], &keepD[i])
		}
	})
	if allocs != 0 {
		t.Fatalf("Pick allocates: %.1f per %d picks", allocs, len(keepS))
	}
}

// BenchmarkHBotCombatPick times one attackers and one blockers Pick of the
// port at positions reached by playing an FDN pairing with the port
// itself on the pool table.
func BenchmarkHBotCombatPick(b *testing.B) {
	c, pool := testCorpus(b)
	set := cardsettest.Set(b, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		b.Fatal(err)
	}
	dirac.Register(tbl)
	bot := hbot.New(HBotFacts(c.Reg, tbl, nil))
	var atkS, blkS dirac.State
	var atkD, blkD dirac.Decision
	haveA, haveB := false, false
	for _, p := range PairingsFrom(pool, 0, 8, 78) {
		m, _, err := SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			b.Fatal(err)
		}
		var decks [2][]dirac.CardID
		for s := 0; s < 2; s++ {
			for _, gc := range m.Decks[s] {
				i, ok := set.Lookup(cardName(gc))
				if !ok {
					b.Fatalf("%s not in the pool", cardName(gc))
				}
				decks[s] = append(decks[s], set.PoolID(i))
			}
		}
		var s dirac.State
		s.Init(tbl, decks[0], decks[1], p.Seed, dirac.Config{})
		var d dirac.Decision
		for n := 0; n < 20000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if !haveA && d.Kind == dirac.DkAttackers && d.N >= 4 {
				atkS.CopyFrom(&s)
				atkD, haveA = d, true
			}
			if !haveB && d.Kind == dirac.DkBlockers && d.N >= 4 {
				blkS.CopyFrom(&s)
				blkD, haveB = d, true
			}
			s.Apply(d.Options[bot.Pick(&s, &d)])
		}
		if haveA && haveB {
			break
		}
	}
	if !haveA || !haveB {
		b.Skip("no attackers/blockers position with >= 4 options")
	}
	b.Run("attackers", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bot.Pick(&atkS, &atkD)
		}
	})
	b.Run("blockers", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			bot.Pick(&blkS, &blkD)
		}
	})
}

// TestHBotTapsHelpersThatMakeNoManaFirst: a helper or tap-cost ask answers with the
// first offer that prints no mana ability, since one that does may be needed for the
// rest of the payment and can't do both (CR 601.2g-h, 602.2b). A granted mana ability
// doesn't count (Enduring Vitality), and a resolution's tap payment asks the same way
// (std-eoe).
func TestHBotTapsHelpersThatMakeNoManaFirst(t *testing.T) {
	const helper = "Choose permanents to tap"
	tapcost := func(also ...hbotCond) hbotCond { return answersWith("choose:tapcost", also...) }
	for _, tc := range []struct {
		pool    string
		seed    uint64
		reached []hbotCond
	}{
		{"std-dsk", 1, []hbotCond{tapcost(promptIs(helper))}},
		{"std-dsk", 1, []hbotCond{tapcost(promptIs(helper), decidesWith("Enduring Vitality"))}},
		{"std-tla", 1, []hbotCond{tapcost(), answersWith("choose:waterbend_generic")}},
		// a resolution's unless cost paid by tapping (pay/unlessflow.go)
		{"std-eoe", 78, []hbotCond{tapcost(promptIs("Tap untapped permanents you control to pay the cost"))}},
	} {
		pinnedHBotGames(t, tc.pool, tc.seed, Config{GorgeAutopaySurface: true, PassElision: true}, func(what string, res HBotParityResult) {
			for _, r := range res.Recs {
				if !r.Agree && (strings.Contains(r.Where, "gorge choose:tapcost") || strings.Contains(r.Where, "gorge choose:waterbend")) {
					t.Errorf("%s: %s", what, r.Where)
				}
			}
		}, tc.reached...)
	}
}

// TestHBotFloatsTheColourTheCardNeeds: a source of several colours tapped by hand
// floats the first colour the card it can help still needs, so the bot goes on to
// cast it (policy.go's chooseManaColour).
func TestHBotFloatsTheColourTheCardNeeds(t *testing.T) {
	for _, tc := range []struct {
		pool string
		pair int
		card string
	}{{"std-tla", 87, "Earth King's Lieutenant"}, {"std-otj", 29, "Snakeskin Veil"}} {
		c := &Corpus{Reg: synthReg(t), decks: map[string][]*cards.Card{}}
		set := cardsettest.Synth(t, tc.pool)
		pool, files, err := SynthPool(c.Reg, set, 1, SynthDecks)
		if err != nil {
			t.Fatal(err)
		}
		c.AddDecks(files)
		mk := func(tbl *kop.Table) KernelBot { return hbot.New(HBotFacts(c.Reg, tbl, nil)) }
		p := PairingsFrom(pool, tc.pair, 1, 78)[0]
		res, err := HBotParityGameOpts(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000, mk, HBotParityOptions{Set: set})
		if err != nil {
			t.Fatalf("%s pair %d: %v", tc.pool, tc.pair, err)
		}
		for _, r := range res.Recs {
			if !r.Agree && strings.Contains(r.Where, "gorge cast("+tc.card) {
				t.Errorf("%s pair %d: %s", tc.pool, tc.pair, r.Where)
			}
		}
	}
}
