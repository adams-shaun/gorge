package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The activate-first lockstep over the choosecardparams pool: symmetric
// decks and a policy that activates Mishra's Research Desk before it plays a
// land. Seed 81 reached a board of four Captivating Caves where the
// kernel's filter-script search (mana_filter.go filtSearch) spent its node
// budget before the Caves' "{1}, {T}: Add one mana of any color" funded the
// red spells gorge's prefix search offers (CR 605.3a, 601.2g): with a fourth
// filter gorge's census is unrelaxable and the search activates the filters
// alone (mana_filter.go relaxedPaidLimit).
var kccpSymDeck = []cardset.Entry{
	{Name: "Riverwheel Sweep", Count: 4}, {Name: "Heroes' Hangout", Count: 4},
	{Name: "Mishra's Research Desk", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4},
	{Name: "Captivating Cave", Count: 4}, {Name: "Island", Count: 3},
	{Name: "Mountain", Count: 3}, {Name: "Plains", Count: 3}, {Name: "Forest", Count: 3},
}

// kccpActivateFirstPolicy activates Mishra's Research Desk, then plays a
// land, then casts; a mode ask takes the first offered mode. Everything else
// random.
func kccpActivateFirstPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DPriority:
			for _, a := range c.D.Options {
				if a.Kind() != ActActivate || c.G == nil {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok && chName(c.G.E.G, o.Obj) == "Mishra's Research Desk" {
					return a
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					return a
				}
			}
		case DModes:
			for _, a := range c.D.Options {
				if a.Kind() == ActMode {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

func TestChoosecardparamsActivateFirstLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosecardparams")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range kccpSymDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	decks := [2][]*cards.Card{deck, deck}
	tbl, _, _, err := set.Matchup(kccpSymDeck, kccpSymDeck)
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range []uint64{81, 128} {
		t.Run("", func(t *testing.T) {
			if seed == 128 {
				// gorge's EnsurePaymentActions offers no hand Llanowar Elves
				// cast while PotentialPaymentPlans prices both from the
				// battlefield Llanowar: a gorge planner inconsistency
				// (agent-20261027T035339Z-83c3893a, "Gorge finding"), not a
				// kernel rules gap.
				t.Skip("seed 128: gorge finding agent-20261027T035339Z-83c3893a (PaymentActions omits the hand Llanowar Elves casts)")
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kccp-a", "kccp-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, kccpActivateFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20})
			CloseSide(k)
			if res.Stall != "" && res.Stall != "turns" { // (the policy never attacks: the 20-turn cap ends the game)
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if len(res.Actions) <= 351 {
				t.Fatalf("seed %d: %d decisions, want past the filter board at 351", seed, len(res.Actions))
			}
			t.Logf("seed %d: %d decisions without divergence (%s)", seed, len(res.Actions), res.Stall)
		})
	}
}
