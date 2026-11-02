package oracle

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/search/hbot"
)

// TestHBotFactsTrick pins the exact trick facts HBotFacts reads from gorge's
// compiled IR to gorge's own FDN census (botpolicy TestTrickCensus).
func TestHBotFactsTrick(t *testing.T) {
	t.Parallel()

	c, _ := testCorpus(t)
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	f := HBotFacts(c.Reg, tbl, nil)
	var got []string
	for i := range f.Cards {
		if f.Cards[i].Instant && f.Cards[i].Trick {
			got = append(got, tbl.Cards[i].Name)
		}
	}
	sort.Strings(got)
	want := []string{
		"Claws Out", "Divine Resilience", "Fake Your Own Death", "Fleeting Flight",
		"Giant Growth", "Snakeskin Veil", "Sure Strike",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("combat tricks = %v, want %v", got, want)
	}
}

// TestHBotParityHold is TestHBotParity with gorge's TimingRules.HoldInstant
// armed on both sides: the port must follow the holding gorge bot in
// lockstep, at TestHBotParity's bars. It also proves the rule fires: at
// least one of pairs 0..3 has a different gorge answer sequence than the
// same pair with the rule off.
func TestHBotParityHold(t *testing.T) {
	t.Parallel()

	c, pool := testCorpus(t)
	type tally struct{ n, agree int }
	kinds := map[string]*tally{}
	ps := PairingsFrom(pool, 0, 12, 78)
	mk := func(tbl *kop.Table) KernelBot {
		return hbot.New(HBotFacts(c.Reg, tbl, nil)).EnableHoldInstant()
	}
	onGorge := map[int][]string{}
	manual := 0
	for _, p := range ps {
		res, err := HBotParityGameOpts(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000, mk, HBotParityOptions{HoldInstant: true})
		if err != nil {
			t.Fatalf("pair %d: %v", p.Index, err)
		}
		if res.Res.Div != nil {
			t.Fatalf("pair %d diverged: %s", p.Index, res.Res.Div)
		}
		manual += res.Manual
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
		for _, r := range res.Recs {
			onGorge[p.Index] = append(onGorge[p.Index], r.Gorge)
		}
	}
	if manual == 0 {
		t.Fatal("HoldInstant took no manual priority decision")
	}
	t.Logf("hold manual decisions %d", manual)
	for _, k := range []string{"priority", "attackers", "blockers", "target"} {
		if x := kinds[k]; x == nil || x.n < 20 {
			t.Fatalf("%s: too few decisions compared: %+v", k, x)
		}
	}
	for k, x := range kinds {
		rate := float64(x.agree) / float64(x.n)
		t.Logf("%-16s %5d %.4f", k, x.n, rate)
		want := 0.98
		if k == "priority" {
			want = 0.99
		}
		if x.n >= 20 && rate < want {
			t.Errorf("%s: agreement %.4f < %.2f over %d decisions", k, rate, want, x.n)
		}
	}
	fired := false
	mkOff := func(tbl *kop.Table) KernelBot { return hbot.New(HBotFacts(c.Reg, tbl, nil)) }
	for _, p := range ps {
		if p.Index > 3 {
			break
		}
		res, err := HBotParityGame(c, p.A, p.B, p.Seed, Config{GorgeAutopaySurface: true, PassElision: true}, 20000, mkOff)
		if err != nil {
			t.Fatalf("pair %d (off): %v", p.Index, err)
		}
		if res.Res.Div != nil {
			t.Fatalf("pair %d (off) diverged: %s", p.Index, res.Res.Div)
		}
		var off []string
		for _, r := range res.Recs {
			off = append(off, r.Gorge)
		}
		if strings.Join(off, "\n") != strings.Join(onGorge[p.Index], "\n") {
			fired = true
		}
	}
	if !fired {
		t.Fatal("HoldInstant never changed gorge's answer sequence over pairs 0..3")
	}
}

// TestHBotHoldPlaysFDNGames is TestHBotPlaysFDNGames with the hold rule
// armed: every answer is an option, every game ends, and no Pick allocates.
func TestHBotHoldPlaysFDNGames(t *testing.T) {
	// serial: testing.AllocsPerRun panics in a parallel test

	c, pool := testCorpus(t)
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	dirac.Register(tbl)
	bot := hbot.New(HBotFacts(c.Reg, tbl, nil)).EnableHoldInstant()
	var keepS []*dirac.State
	var keepD []dirac.Decision
	for _, p := range PairingsFrom(pool, 0, 8, 78) {
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
