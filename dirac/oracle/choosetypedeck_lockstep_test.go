package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The mid-resolution CreatureInTargetedDeck AtRandom$ choose (CR 611.3a; gorge
// effects/choose.go effChooseType's AtRandom arm): Aswan Jaguar's enters
// trigger poses "target opponent", then draws a random creature type from
// those its library's cards print — no player is asked, the draw is one
// rng.IntN over the sorted list when it holds two or more words — and stores
// it as the source's chosen type. The destroy ability's
// ValidTgts$ Creature.ChosenType target then reads that choice, so a creature
// of the drawn type must be targetable. Both decks are the same creature-heavy
// build, so every drawn type (Cat, Bear, Faerie) has targets on both boards.

// kchooseDeck is the pool's one deck: Aswan Jaguar plus creatures of three
// subtypes, spells and forests.
var kchooseDeck = []cardset.Entry{
	{Name: "Aswan Jaguar", Count: 4},
	{Name: "Grizzly Bears", Count: 4},
	{Name: "Savannah Lions", Count: 4},
	{Name: "Scryb Sprites", Count: 4},
	{Name: "Giant Growth", Count: 8},
	{Name: "Forest", Count: 16},
}

// TestChooseTypeRandomDeckCreature plays the pool in lockstep and asserts the
// choose was actually reached: the coverage credits Aswan Jaguar's ETB trigger
// (the choose runs inside it) and its destroy ability (the reader) over the
// run. No gorge decision is posed by the AtRandom draw, so the Check only
// lets the comparison run; a divergence anywhere in the resolution or the
// reader fails the seed.
func TestChooseTypeRandomDeckCreature(t *testing.T) {
	t.Parallel()
	reg, set, gd, tbl := chooseTypeFixture(t)
	cast := map[string]bool{}
	used := map[string]int{}
	for seed := uint64(1); seed <= 120; seed++ {
		m, err := NewMatchup(reg, [2]string{"choosetype-a", "choosetype-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%4 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40,
			Finish: func(g *GorgeSide) {
				if g.E == nil {
					return
				}
				c := CoverageOf(g, m)
				if cc := c.Cards["Aswan Jaguar"]; cc != nil {
					if cc.Cast > 0 {
						cast["Aswan Jaguar"] = true
					}
					used["trigger"] += cc.Used["f0/t0"]
					used["destroy"] += cc.Used["f0/a0"]
				}
			}})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
			continue
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			continue
		}
	}
	if !cast["Aswan Jaguar"] {
		t.Fatal("precondition: Aswan Jaguar was never cast in 120 seeds, so its choose was never compared")
	}
	if used["trigger"] == 0 {
		t.Fatal("precondition: Aswan Jaguar's ETB trigger never resolved, so the random draw was never compared")
	}
	if used["destroy"] == 0 {
		t.Fatal("precondition: the destroy ability was never activated, so the ChosenType reader was never compared")
	}
	t.Logf("choosetype: trigger %d, destroy %d", used["trigger"], used["destroy"])
}

// chooseTypeFixture builds the choosetypedeck pool's matchup: both seats play
// the one 40-card deck.
func chooseTypeFixture(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosetypedeck")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := 0; s < 2; s++ {
		for _, e := range kchooseDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards, want 40", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kchooseDeck, kchooseDeck)
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, gd, tbl
}
