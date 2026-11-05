package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Q, Stacking the Deck lockstep: the card Q's enters trigger exiles from
// the top of the library gains "Exile a card from your hand: Create a copy of
// this card. You may cast the copy" for as long as it stays exiled, usable on
// its owner's turn. The kernel's exile grant, its hand-exile cost pick and its
// paid copy cast must offer, ask and play exactly what gorge's exile granted
// walk does at every decision.

var kqdeckDecks = [2][]cardset.Entry{
	{
		{Name: "Q, Stacking the Deck", Count: 4}, {Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 6},
		{Name: "Mountain", Count: 22},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 6}, {Name: "Mountain", Count: 26},
	},
}

// qdeckPolicy casts and activates whenever it can, including a granted exile
// ability whose copy it cannot pay: gorge begins that cast, taps what its mana
// window offers and reverses the cast (CR 733.1), the taps and their mana
// standing. Every land is a Mountain, so a copy is payable exactly when the
// untapped lands and the pool hold its mana value; the unpayable activations
// are counted under qdeckUnpayable for the test's precondition.
const qdeckUnpayable = "__unpayable__"

func qdeckPolicy(seed uint64, acts map[string]int) Policy {
	inner := poisonhighPolicy(seed, map[string]int{}, acts)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return inner.Pick(c)
		}
		for _, a := range c.D.Options {
			if a.Kind() != ActActivate {
				continue
			}
			if o, ok := c.G.GorgeOption(a); ok && o.Kind == "ability" && o.SVar != "" {
				g := c.G.E.G
				ob := g.Obj(o.Obj)
				if ob == nil || ob.Zone != state.ZExile || ob.Face() == nil {
					continue
				}
				mana := int(g.Players[o.Player].Pool.Total())
				for _, id := range g.Zone(state.ZBattlefield, o.Player) {
					if l := g.Obj(id); l != nil && l.Face() != nil && l.Face().IsLand() && !l.Tapped {
						mana++
					}
				}
				if mana < int(ob.Face().ManaValue()) {
					acts[qdeckUnpayable]++
				}
			}
		}
		return inner.Pick(c)
	})
}

// runQDeck plays seeds 1..seeds and returns the games, divergences and stalls.
func runQDeck(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, acts map[string]int) {
	t.Helper()
	acts = map[string]int{}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("qdeck")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugQStackNoGrant
	dirac.DebugQStackNoGrant = mutate
	defer func() { dirac.DebugQStackNoGrant = old }()
	var decks [2][]*cards.Card
	for s, d := range kqdeckDecks {
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
	tbl, _, _, err := set.Matchup(kqdeckDecks[0], kqdeckDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kqdeck-a", "kqdeck-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := qdeckPolicy(seed, acts)
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return
}

func TestQStackingTheDeckLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, acts := runQDeck(t, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, activations by card %v", games, divs, stalls, acts)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	// the granted ability is activated (its source is the exiled card)
	n := acts["Hill Giant"] + acts["Shock"]
	if n < 20 {
		t.Errorf("only %d activations of a granted exile ability (want at least 20)", n)
	}
}

// TestQStackingTheDeckMutation drops the grant: the games must diverge. A
// green mutation (no divergence) means the lockstep never reaches the ability.
func TestQStackingTheDeckMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runQDeck(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation dropped every grant in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
