package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCValues1Lowers is the lockstep game per spelling of the MBC
// values-1 family: a pool built by kgen from the card's own decks (the build
// fails, and this test with it, while the spelling is refused), a policy that
// drives toward the card, and a tally of the behaviour the spelling lowers.
//
// Dyfed, the Guiding Hand lowers a planeswalker's [-X]: the announced X is
// the target bounds of "Untap X target artifacts", offered only while the
// target ask could answer it (CR 601.2b, 601.2c). Triumph of Ferocity lowers
// the ConditionCheckSVar gate beside the present check: draw at upkeep when
// you control the creature with the greatest power or tied for it. Pir,
// Imaginative Rascal lowers the any-kind plus-one counter replacement, the
// loyalty cost's counters included (Jace Beleren's [+2] puts three).
//
// Sahir, Visitor in Darkness and Lae'zel, Vlaakith's Champion stay refused by
// kgen for blockers outside this family (Sahir's trigger-matcher params,
// Lae'zel's Choose a Background and the You recipient of its putter line), so
// they have no game here.
var mbcValues1Decks = [3][2][]cardset.Entry{
	{ // M1: Dyfed beside artifacts to untap (Ornithopter is a free artifact, the Powerstone tokens are made)
		{
			{Name: "Dyfed, the Guiding Hand", Count: 6}, {Name: "Ornithopter", Count: 8},
			{Name: "Island", Count: 16}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Raging Goblin", Count: 8},
			{Name: "Doom Blade", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 8},
		},
	},
	{ // M2: Triumph of Ferocity beside creatures of several powers on both sides
		{
			{Name: "Triumph of Ferocity", Count: 6}, {Name: "Hill Giant", Count: 6},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Raging Goblin", Count: 6},
			{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 6},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Raging Goblin", Count: 6}, {Name: "Hill Giant", Count: 6},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
		},
	},
	{ // M3: Pir, Imaginative Rascal beside counter makers of several shapes:
		// the greenhouse's one +1/+1 (plus one is two), Jace Beleren's [+2]
		// loyalty cost (three loyalty counters), the pantry's charge counters
		{
			{Name: "Pir, Imaginative Rascal", Count: 6}, {Name: "Atmospheric Greenhouse", Count: 3},
			{Name: "Stocking the Pantry", Count: 3}, {Name: "Wingspan Mentor", Count: 4},
			{Name: "Jace Beleren", Count: 3}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Island", Count: 8}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Ajani, Caller of the Pride", Count: 4}, {Name: "Walking Corpse", Count: 6},
			{Name: "Doom Blade", Count: 4}, {Name: "Murder", Count: 3},
			{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 11},
		},
	},
}

// mbcValues1Stats is what the gorge-side event log says the spelling decided.
type mbcValues1Stats struct {
	plays  int // casts of the card / activations of its lowered ability
	unTaps int // Dyfed: artifacts untapped while Dyfed is in play (the [-X])
	minusX int // Dyfed: loyalty counters removed (the [-X] cost, X >= 1)
	draws  int // Triumph: draws while Triumph is in play (the upkeep trigger)
	ctr    int // Pir: counter placements while Pir is in play (the plus one)
	loyal3 int // Pir: three loyalty counters at once (Jace's [+2] cost plus one)
	acts   int // Dyfed: activations of any of its abilities (debug)
	// abilSeen is the ability indices the Dyfed activation options carry (debug).
	abilSeen map[int]int
	noArt    int      // Dyfed: [-X] picks with no artifact on the battlefield (debug)
	dyfCtr   int      // Dyfed: counter events on a Dyfed permanent (debug)
	loyalAny int      // any LOYALTY counter event (debug)
	negAny   int      // any negative counter event (debug)
	seed1    []string // matchup 0 seed 1's counter events (debug)
}

// mbcValues1Tally is the event-log scanner one matchup keeps between seeds.
type mbcValues1Tally struct {
	evAt int
}

func (p *mbcValues1Tally) scan(ref *GorgeSide, mi, seed int, st *mbcValues1Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Draw:
			if e.To != state.ZHand {
				continue
			}
			if battlefieldHasName(g, e.Player, "Triumph of Ferocity") {
				st.draws++
			}
		case events.CounterChange:
			// The event log is read at the end of the game, so an object's
			// zone is read as it stands NOW: the permanent a counter event
			// names is identified by its face, not by where it lies.
			o := g.Obj(e.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			if o.Face().Name == "Dyfed, the Guiding Hand" && e.Counter == "LOYALTY" && e.Amount <= -1 {
				st.minusX++
			}
			if e.Amount <= 0 || o.Zone != state.ZBattlefield {
				continue
			}
			if battlefieldHasName(g, o.Controller, "Pir, Imaginative Rascal") {
				st.ctr++
				if e.Counter == "LOYALTY" && e.Amount == 3 {
					st.loyal3++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// mbcValues1Names is the card-name list the test's kernel table is built from.
func mbcValues1Names(t *testing.T) [2][]string {
	t.Helper()
	var names [2][]string
	for _, d := range mbcValues1Decks {
		for s, dd := range d {
			for _, e := range dd {
				for i := 0; i < e.Count; i++ {
					names[s] = append(names[s], e.Name)
				}
			}
		}
	}
	return names
}

// mbcValues1Policy plays toward the matchup's card: it casts the card, lands
// and (Dyfed) the [-X] ability ahead of anything else, and answers everything
// else at random. The plays it takes are tallied.
func mbcValues1Policy(seed uint64, card string, actAbility int, st *mbcValues1Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6d62633431)) // "mbc41"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			best, bestRank := Canon(0), 0
			for _, o := range c.D.Options {
				r := 0
				g, ok := c.G.GorgeOption(o)
				switch {
				case !ok:
				case o.Kind() == ActCast && g.Kind == "cast" && sacDiscountSource(c.G, g.Obj) == card:
					r = 6
				case o.Kind() == ActActivate && g.Kind == "ability" && actAbility >= 0 &&
					sacDiscountSource(c.G, g.Obj) == card && int(g.Ability) == actAbility:
					r = 5
				}
				if r > bestRank {
					bestRank, best = r, o
				}
			}
			if bestRank >= 5 {
				st.plays++
				return best
			}
		}
		switch c.D.Kind {
		case DPriority:
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		}
		return rnd.Pick(c)
	})
}

// runMBCValues1 plays each matchup's lockstep games and returns the games,
// divergences and tallies.
func runMBCValues1(t *testing.T, seeds int) (games, divs int, st [3]mbcValues1Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set := printedDelveSet(t, mbcValues1Names(t))
	names := mbcValues1Decks
	cardNames := []string{"Dyfed, the Guiding Hand", "Triumph of Ferocity", "Pir, Imaginative Rascal"}
	abilities := [3]int{-1, -1, -1}
	abilities[0] = 1 // Dyfed's [-X]: the second printed ability
	for mi, md := range names {
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
		var tl mbcValues1Tally
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{}
			if seed%4 == 0 {
				cfg = Config{GorgeAutopaySurface: true}
			}
			m, err := NewMatchup(reg, [2]string{"mbcvals1-a", "mbcvals1-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, mbcValues1Policy(seed, cardNames[mi], abilities[mi], &st[mi]),
				RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			tl.scan(g, mi, int(seed), &st[mi])
			games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
				continue
			}
			if d := res.Div; d != nil {
				divs++
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
					mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, st
}

func TestMBCValues1Lowers(t *testing.T) {
	t.Parallel()
	games, divs, st := runMBCValues1(t, 24)
	t.Logf("%d games, %d divergences", games, divs)
	t.Logf("Dyfed %+v Triumph %+v Pir %+v", st[0], st[1], st[2])
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Dyfed cast or its [-X] activated", st[0].plays},
		{"Dyfed [-X] costs paid with X >= 1", st[0].minusX},
		{"Triumph cast", st[1].plays},
		{"draws under Triumph", st[1].draws},
		{"Pir cast", st[2].plays},
		{"counter placements under Pir", st[2].ctr},
		{"three loyalty counters at once under Pir (Jace's [+2])", st[2].loyal3},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}
