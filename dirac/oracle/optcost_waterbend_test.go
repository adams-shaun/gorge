package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The OptionalCost Waterbend<N> lockstep: "as an additional cost to cast this
// spell, you may waterbend {N}" (CR 701.67a, 601.2b; Ruinous Waterbending's
// {4}, Spirit Water Revival's {6}). gorge offers the optional cast as its own
// priority option (Mode "optionalcost") priced with the waterbend offer credit
// (mana.go withWaterbendOfferCredit), asks the waterbend taps at payment
// (pay.ConvokeAsk, "waterbend_generic") and stamps FlagOptionalCostPaid, which
// Condition$ OptionalCost and Count$OptionalGenericCostPaid read. The kernel's
// optional cast is the kicked one, its {N} in CardDef.Kicker and its taps the
// spell waterbend pick (act_waterbend.go). gorge's planner prices no
// OptionalCost card, so the optional cast is offered while the floating pool
// pays the credited cost: Priest of Gix's BBB, Rapturous Moment's UURRR.

var koptwaterDecks = [2][]cardset.Entry{
	{
		{Name: "Ruinous Waterbending", Count: 6}, {Name: "Priest of Gix", Count: 6},
		{Name: "Memnite", Count: 4}, {Name: "Ornithopter", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Swamp", Count: 12}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Spirit Water Revival", Count: 6}, {Name: "Rapturous Moment", Count: 6},
		{Name: "Memnite", Count: 4}, {Name: "Ornithopter", Count: 4}, {Name: "Gray Ogre", Count: 2},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 8},
	},
}

var koptwaterCarriers = map[string]bool{"Ruinous Waterbending": true, "Spirit Water Revival": true}

// koptwaterTally counts, per carrier, the casts the policy chose and the
// waterbend taps it announced.
type koptwaterTally struct {
	paid, plain map[string]int
	taps        int
}

// koptwaterPolicy takes an offered optional-cost cast two times in three,
// else a cast (so the plain carrier cast is picked too), and announces every
// waterbend tap offered except on some seeds, where it closes early.
func koptwaterPolicy(seed uint64, tl *koptwaterTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6f707477))
	cp, haveWB := chooseCode("waterbend_generic")
	var opt, casts []Canon
	pick := func(c *PickCtx, from []Canon) Canon {
		d := *c.D
		d.Options = from
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		opt, casts = opt[:0], casts[:0]
		var wb, done Canon
		haveTap, haveDone := false, false
		for _, a := range c.D.Options {
			switch {
			case a.Kind() == ActCast:
				casts = append(casts, a)
				if c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok && o.Mode == "optionalcost" {
						opt = append(opt, a)
					}
				}
			case haveWB && a.Kind() == ActChoose+CanonKind(cp) && !haveTap:
				wb, haveTap = a, true
			case a.Kind() == ActDone:
				done, haveDone = a, true
			}
		}
		if haveTap {
			if haveDone && seed%3 == 1 && r.IntN(2) == 0 {
				return done
			}
			tl.taps++
			return wb
		}
		var a Canon
		switch {
		case len(casts) == 0:
			a = rnd.Pick(c)
		case len(opt) > 0 && r.IntN(3) != 0:
			a = pick(c, opt)
		default:
			a = pick(c, casts)
		}
		if c.G == nil || a.Kind() != ActCast {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok || o.Kind != "cast" {
			return a
		}
		name := c.G.E.G.Obj(o.Obj).Face().Name
		if !koptwaterCarriers[name] {
			return a
		}
		switch o.Mode {
		case "optionalcost":
			tl.paid[name]++
		case "":
			tl.plain[name]++
		}
		return a
	})
}

// runOptWaterbend plays seeds 1..seeds and returns the games, divergences,
// kernel capacity stalls and the tally; mutate sets DebugOptCostNeverPaid.
func runOptWaterbend(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl koptwaterTally) {
	t.Helper()
	tl = koptwaterTally{paid: map[string]int{}, plain: map[string]int{}}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("optwater")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugOptCostNeverPaid
	dirac.DebugOptCostNeverPaid = mutate
	defer func() { dirac.DebugOptCostNeverPaid = old }()
	md := koptwaterDecks
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"koptwater-a", "koptwater-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, koptwaterPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
		games++
		CloseSide(k)
		if d := res.Div; d != nil {
			divs++
			if !mutate && divs <= 3 {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return games, divs, stalls, tl
}

func TestOptCostWaterbendLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runOptWaterbend(t, 160, false)
	t.Logf("%d games, %d divergences, %d stalls, paid %v, plain %v, waterbend taps %d", games, divs, stalls, tl.paid, tl.plain, tl.taps)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	// the casts the lockstep compared: each carrier paid and plain, with taps
	for name := range koptwaterCarriers {
		if tl.paid[name] == 0 || tl.plain[name] == 0 {
			t.Errorf("%s: %d paid, %d plain casts; want both", name, tl.paid[name], tl.plain[name])
		}
	}
	if tl.taps == 0 {
		t.Error("no waterbend tap was announced")
	}
	// the paid flag reaches the resolution: reading it as unpaid diverges
	if _, mdivs, _, _ := runOptWaterbend(t, 160, true); mdivs == 0 {
		t.Error("DebugOptCostNeverPaid: no divergence; the paid branch went unread")
	}
}
