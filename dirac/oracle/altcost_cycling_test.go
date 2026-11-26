package oracle

import (
	"math/rand/v2"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// The cycling alternative cost lockstep (CR 118.9): New Perspectives' "As long
// as you have seven or more cards in hand, you may pay {0} rather than pay
// cycling costs" beside a plain cycler (Lonely Sandbar), a typecycler
// (Eternal Dragon) and a pay-life cycler (Street Wraith). The enters trigger
// draws three, so the games reach seven cards in hand and then fall back
// below it as the hand is cast and cycled. gorge's option (an "ability" at
// AltCostIndex 1) and the kernel's twin ability meet in one canonical
// activation, offered at seven and absent at six. The expected answer at
// every decision is gorge's.
var kcyclingaltDecks = [2][]cardset.Entry{
	{
		{Name: "New Perspectives", Count: 6}, {Name: "Lonely Sandbar", Count: 6}, {Name: "Daggermaw Megalodon", Count: 2}, {Name: "Jhessian Zombies", Count: 2},
		{Name: "Street Wraith", Count: 4}, {Name: "Island", Count: 14}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 16}, {Name: "Forest", Count: 24},
	},
}

// cyclingAltTally is what the policy saw: alternative activations offered with
// seven or more cards in hand, and printed cycling activations offered beside
// New Perspectives with exactly six (no alternative then), each checked
// against gorge's own hand before the pick.
type cyclingAltTally struct{ atSeven, atSix, taken atomic.Int64 }

// cyclingAltPolicy is castFirstPolicy biased to the cycling activations (the
// alternative one in 3 of 4 picks when offered, any activation in 1 of 2), and
// asserts the hand size the offer depends on from gorge's side.
func cyclingAltPolicy(t *testing.T, tally *cyclingAltTally) func(uint64) Policy {
	return func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		r := rand.New(rand.NewPCG(seed, 0xa17c))
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G == nil || c.D.Kind != DPriority {
				return base.Pick(c)
			}
			hand := len(c.G.E.G.Zone(state.ZHand, state.PlayerID(c.D.Player)))
			var alt, plain []Canon
			for _, x := range c.D.Options {
				if x.Kind() != ActActivate || x.Mode() >= maxSpeedBase {
					continue
				}
				if x.Mode()&cyclingAltBit != 0 {
					alt = append(alt, x)
				} else {
					plain = append(plain, x)
				}
			}
			// gorge's own view of the offer: hand cycling options, printed and alternative
			printedOpts, altOpts := 0, 0
			if d := c.G.E.Pending(); d != nil {
				for _, o := range d.Options {
					if ob := c.G.E.G.Obj(o.Obj); o.Kind == "ability" && ob != nil && ob.Zone == state.ZHand {
						if o.AltCostIndex == 0 {
							printedOpts++
						} else {
							altOpts++
						}
					}
				}
			}
			if altOpts != len(alt) {
				t.Errorf("decision %d: gorge offers %d alternative cyclings, the canonical decision %d", c.Seq, altOpts, len(alt))
			}
			if len(alt) > 0 {
				if hand < 7 {
					t.Errorf("decision %d: an alternative cycling offered with %d cards in hand", c.Seq, hand)
				}
				tally.atSeven.Add(1)
				if r.IntN(4) != 0 {
					tally.taken.Add(1)
					return alt[r.IntN(len(alt))]
				}
			}
			if printedOpts > 0 && altOpts == 0 && hand == 6 && newPerspectivesIn(c.G, state.PlayerID(c.D.Player)) {
				tally.atSix.Add(1) // the printed cycling alone: six cards in hand, the static in force on the board
			}
			if act := append(alt, plain...); len(act) > 0 && r.IntN(2) == 0 {
				return act[r.IntN(len(act))]
			}
			return base.Pick(c)
		})
	}
}

// newPerspectivesIn reports whether seat p controls a New Perspectives.
func newPerspectivesIn(g *GorgeSide, p state.PlayerID) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "New Perspectives" {
			return true
		}
	}
	return false
}

func TestCyclingAltCostLockstep(t *testing.T) {
	_, set, gd := lockstepPool(t, "cyclingalt", kcyclingaltDecks)
	if !SetDeckSupported(set, gd[0]) || !SetDeckSupported(set, gd[1]) {
		t.Fatal("the cycling alternative decks are not playable in the cyclingalt pool")
	}
	var tally cyclingAltTally
	lockstepRunPolicy(t, "cyclingalt", kcyclingaltDecks, 200, true, nil, nil, cyclingAltPolicy(t, &tally))
	if tally.atSeven.Load() == 0 || tally.atSix.Load() == 0 || tally.taken.Load() == 0 {
		t.Fatalf("the pool does not reach the offer: %d offers at seven cards, %d plain cyclings at six, %d alternatives taken",
			tally.atSeven.Load(), tally.atSix.Load(), tally.taken.Load())
	}
	t.Logf("alternative offers at seven: %d, taken %d; plain cyclings at six: %d", tally.atSeven.Load(), tally.taken.Load(), tally.atSix.Load())
}

// TestCyclingAltCostLockstepMutation: with every twin withheld from the
// kernel's table, the seeds that reach the offer diverge.
func TestCyclingAltCostLockstepMutation(t *testing.T) {
	var tally cyclingAltTally
	mutate := func(tb *kop.Table) {
		for i := range tb.Cards {
			for j := range tb.Cards[i].Abilities {
				if tb.Cards[i].Abilities[j].CycAltOf != 0 {
					tb.Cards[i].Abilities[j].Kind = kop.AbStatic // never an activation
					tb.Cards[i].Abilities[j].Static = kop.Static{}
				}
			}
		}
	}
	if lockstepRunPolicy(t, "cyclingalt", kcyclingaltDecks, 200, false, mutate, nil, cyclingAltPolicy(t, &tally)) > 0 {
		return
	}
	t.Fatal("no seed diverged with the cycling twins withheld: the pool does not exercise the alternative")
}
