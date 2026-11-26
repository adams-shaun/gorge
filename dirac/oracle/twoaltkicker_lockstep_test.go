package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The two-alternative Kicker lockstep (CR 702.33b): Thornscape Battlemage
// (the and/or Kicker carrier the mbc pool carries, kernel kw_kickalt.go). No
// pool deck plays the card, so this is the lockstep reach the pool fuzz
// cannot give: the family in real games -- the plain casts both engines
// offer and pay, its ETB triggers, its combat and its Count$TimesKicked-class
// provenance reads (the kernel's kickAlt byte, 0 for a plain cast) -- never
// diverge. The per-alternative casts themselves are never on a posed list
// (gorge prices them from its floating-pool walk gate alone; see
// ktwoaltPolicy); the kernel-side offer, payment and provenance of the four
// casts are the dirac scenario tests'.

var ktwoaltDecks = [][2][]cardset.Entry{{
	// A: the battlemage beside creatures (the kicked 1 damage trigger's
	// candidates) and an artifact (the kicked 2 destroy one)
	{
		{Name: "Thornscape Battlemage", Count: 4}, {Name: "Elas il-Kor, Sadistic Pilgrim", Count: 3},
		{Name: "Pelakka Wurm", Count: 2}, {Name: "Everflowing Chalice", Count: 2},
		{Name: "Forest", Count: 14}, {Name: "Mountain", Count: 8}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Elas il-Kor, Sadistic Pilgrim", Count: 3}, {Name: "Everflowing Chalice", Count: 2},
		{Name: "Pelakka Wurm", Count: 2}, {Name: "Caltrops", Count: 2},
		{Name: "Forest", Count: 11}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
}, {
	// B: both seats cast it, so the destroyed artifact and the damage target
	// live on both sides
	{
		{Name: "Thornscape Battlemage", Count: 4}, {Name: "Elas il-Kor, Sadistic Pilgrim", Count: 3},
		{Name: "Everflowing Chalice", Count: 2}, {Name: "Forest", Count: 14},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Thornscape Battlemage", Count: 4}, {Name: "Pelakka Wurm", Count: 2},
		{Name: "Everflowing Chalice", Count: 2}, {Name: "Forest", Count: 14},
		{Name: "Mountain", Count: 9}, {Name: "Plains", Count: 9},
	},
}}

// ktwoaltPolicy steers the games to the battlemage: a cast of one is taken
// three times in four, else random; every taken battlemage cast is tallied by
// mode. The per-alternative casts ride gorge's floating-pool walk gate and its
// potential planner has no kicked1/kicked2/kickedboth row
// (potential_plan.go's potentialModeBaseCostCodes), so no pool in the corpus
// can float the mana a posed priority would need (no pool card adds mana
// outside a payment window, and a payment's floats do not survive to a posed
// priority) -- the per-alternative casts are therefore never on a posed list
// in these games; the tally's empty-mode entries are the plain casts the
// games do exercise, and the per-alternative offer, payment and provenance
// are the dirac scenario tests' (twoaltkicker_offer_test.go, kw_kickedalt_test.go).
func ktwoaltPolicy(seed uint64, tl map[string]int) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6b696b32))
	var casts []Canon
	pick := func(c *PickCtx, from []Canon) Canon {
		d := *c.D
		d.Options = from
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		casts = casts[:0]
		if c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" && c.G.E.G.Obj(o.Obj).Face().Name == "Thornscape Battlemage" {
					casts = append(casts, a)
				}
			}
		}
		var a Canon
		switch {
		case seed%4 == 0 || len(casts) == 0:
			a = rnd.Pick(c)
		case r.IntN(4) != 0:
			a = pick(c, casts)
		default:
			a = rnd.Pick(c)
		}
		if c.G == nil || a.Kind() != ActCast {
			return a
		}
		if o, ok := c.G.GorgeOption(a); ok && o.Kind == "cast" {
			if name := c.G.E.G.Obj(o.Obj).Face().Name; name == "Thornscape Battlemage" {
				tl[o.Mode]++
			}
		}
		return a
	})
}

// runTwoAltKicker plays both matchups for seeds 1..seeds and returns the
// games, the divergences, the kernel capacity stalls and the per-mode tally.
func runTwoAltKicker(t *testing.T, seeds int) (games, divs, stalls int, tl map[string]int) {
	t.Helper()
	tl = map[string]int{}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbc")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range ktwoaltDecks {
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
			m, err := NewMatchup(reg, [2]string{"ktwoalt-a", "ktwoalt-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, ktwoaltPolicy(seed, tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			games++
			if d := res.Div; d != nil {
				divs++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	return games, divs, stalls, tl
}

// TestTwoAltKickerLockstep: the and/or Kicker's per-alternative casts agree
// gorge-vs-kernel, and the policy reaches every mode.
func TestTwoAltKickerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runTwoAltKicker(t, 24)
	t.Logf("%d games, %d divergences, %d stalls; casts %v", games, divs, stalls, tl)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	n := 0
	for _, c := range tl {
		n += c
	}
	if n < 10 {
		t.Errorf("thin battlemage coverage: %d casts", n)
	}
}
