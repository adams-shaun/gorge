package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Shuffle Defined$ RememberedOwner lockstep (CR 108.3, 701.24): Elixir of
// Immortality moves itself and your graveyard into their owners' libraries
// with RememberChanged$, then every distinct owner of a remembered card
// shuffles. Steal Artifact lets the opponent's Elixir be activated by a
// controller who does not own it, so the Elixir's owner and the graveyard's
// owner differ. Both seats play the same deck over the shuffleowners pool.

var shuffleOwnersDeck = []cardset.Entry{
	{Name: "Elixir of Immortality", Count: 6}, {Name: "Steal Artifact", Count: 4},
	{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 4},
	{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 8},
}

// runShuffleOwners plays the seeded games; mutate makes the Elixir's shuffle
// name the opponent's library, which must diverge. It counts the Elixirs that
// reached a library from the battlefield, and those a non-owner activated.
func runShuffleOwners(t *testing.T, seeds uint64, mutate bool) (games, divs int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("shuffleowners")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = shuffleOwnersDeck
		for _, e := range shuffleOwnersDeck {
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
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		cleared := 0
		for ci := range tbl.Cards {
			for ai := range tbl.Cards[ci].Abilities {
				for pi := range tbl.Cards[ci].Abilities[ai].Prog {
					in := &tbl.Cards[ci].Abilities[ai].Prog[pi]
					if in.Op == kop.OpShuffle && in.Obj == kop.RefRememberedOwner {
						in.Obj, in.C = kop.RefOpponent, 0
						cleared++
					}
				}
			}
		}
		if cleared == 0 {
			t.Fatal("mutation found no Shuffle Defined$ RememberedOwner in the pool")
		}
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kso-a", "kso-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if gs != nil {
			var prev events.Event
			for _, ev := range gs.E.L.Events {
				if ev.Kind == events.Shuffle && prev.Kind == events.Shuffle && prev.Player != ev.Player {
					branch["two owners shuffled"]++ // Elixir and the graveyard had different owners
				}
				prev = ev
				if ev.Kind != events.MoveZone || ev.From != state.ZBattlefield || ev.To != state.ZLibrary {
					continue
				}
				ob := gs.E.G.Obj(ev.Obj)
				if ob == nil || ob.Face() == nil || ob.Face().Name != "Elixir of Immortality" {
					continue
				}
				branch["elixir shuffled in"]++
			}
		}
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestShuffleOwnersLockstep(t *testing.T) {
	t.Parallel()

	games, divs, branch := runShuffleOwners(t, 300, false)
	t.Logf("%d games, %d divergences; %v", games, divs, branch)
	if branch["elixir shuffled in"] < 8 {
		t.Errorf("Elixir shuffled itself into a library %d times, want at least 8", branch["elixir shuffled in"])
	}
	if branch["two owners shuffled"] == 0 {
		t.Errorf("no activation shuffled two owners' libraries (a stolen Elixir)")
	}
}

// TestShuffleOwnersMutation proves the pool's shuffle matters: shuffling the
// opponent's library instead of the remembered owners' diverges.
func TestShuffleOwnersMutation(t *testing.T) {
	t.Parallel()

	games, divs, _ := runShuffleOwners(t, 300, true)
	if divs == 0 {
		t.Fatalf("mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
