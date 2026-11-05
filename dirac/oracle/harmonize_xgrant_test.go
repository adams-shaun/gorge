package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The {X} Harmonize lockstep (CR 702.180a-b): Nature's Rhythm harmonized
// from the graveyard announces the one creature tapped for its power before
// X (gorge convokeAsk then xAsk), so the X ask prices the pick: an X the
// tapped creature's power would reduce nothing at is not offered, and the
// ceiling rises by that power. The expected answer at every decision is
// gorge's. (Songcrafter Mage's granted harmonize stays refused: gorge offers
// a harmonize cast only off the printed keyword, legal_walk_alt.go
// graveyardCastsOver.)

var harmonizeXDecks = [2][]cardset.Entry{
	{ // A: G/U/R, the carrier
		{Name: "Nature's Rhythm", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mulch", Count: 2}, {Name: "Tormenting Voice", Count: 2}, {Name: "Satyr Wayfinder", Count: 2},
		{Name: "Giant Growth", Count: 2}, {Name: "Shock", Count: 2}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 3},
	},
	{ // B: R/G
		{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Giant Growth", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 2}, {Name: "Fire Elemental", Count: 2},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

type harmonizeXStats struct {
	casts, pickedX, pickedX2 int
}

// harmonizeXProbe is a seeded random policy biased to cast, to take a
// harmonize cast, to tap a creature at the announcement and to announce the
// largest X, counting on gorge's side the Nature's Rhythm harmonize casts and
// the X announcements made after a creature was tapped.
type harmonizeXProbe struct {
	st     *harmonizeXStats
	base   Policy
	r      *rand.Rand
	rhythm bool // a Nature's Rhythm harmonize cast is being announced
	picked bool // ... and its announcement tapped a creature
}

func (p *harmonizeXProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	var harm, casts, picks []Canon
	var xTop Canon
	top := -1
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		switch {
		case !ok:
		case x.Kind() == ActCast && o.Mode == "harmonize":
			harm = append(harm, x)
		case x.Kind() == ActCast:
			casts = append(casts, x)
		case o.Kind == "harmonize":
			picks = append(picks, x)
		case o.Kind == "x" && o.Amount > top:
			top, xTop = o.Amount, x
		}
	}
	switch {
	case len(picks) > 0:
		if p.r.IntN(4) != 0 {
			a = picks[p.r.IntN(len(picks))]
		}
		if o, ok := c.G.GorgeOption(a); ok && o.Kind == "harmonize" && p.rhythm {
			p.picked = true
		}
	case top >= 0:
		if p.rhythm && p.r.IntN(2) == 0 {
			a = xTop
		}
		if o, ok := c.G.GorgeOption(a); ok && p.rhythm && p.picked {
			p.st.pickedX++
			if o.Amount >= 2 {
				p.st.pickedX2++
			}
		}
		p.rhythm, p.picked = false, false
	case len(harm) > 0 && p.r.IntN(8) != 0:
		a = harm[p.r.IntN(len(harm))]
	case len(casts) > 0 && p.r.IntN(4) != 0:
		a = casts[p.r.IntN(len(casts))]
	}
	if a.Kind() == ActCast {
		o, ok := c.G.GorgeOption(a)
		p.rhythm = ok && o.Mode == "harmonize" && chName(c.G.E.G, o.Obj) == "Nature's Rhythm"
		p.picked = false
		if p.rhythm {
			p.st.casts++
		}
	}
	return a
}

func TestHarmonizeXGrantLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("harmonizex")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range harmonizeXDecks {
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
	tbl, _, _, err := set.Matchup(harmonizeXDecks[0], harmonizeXDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st harmonizeXStats
	const seeds = 384
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"harmonizex-a", "harmonizex-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &harmonizeXProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x68786772))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 24})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		} else if res.Stall != "" && res.Stall != "turns" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %+v", seeds, st)
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.casts, 10, "Nature's Rhythm harmonize casts"},
		{st.pickedX, 5, "X announcements after a creature was tapped"},
		{st.pickedX2, 3, "... with X >= 2"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}
