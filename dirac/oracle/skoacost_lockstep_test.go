package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// The Skoa, Embermage lockstep: "Discard another card named Skoa, Embermage,
// Sacrifice two Mountains: Skoa deals 4 damage to any target" is an activated
// cost of a filtered discard beside a counted sacrifice (CR 118.3, 602.2b).
// gorge asks the sacrifice, then the discard over the hand cards named Skoa;
// the kernel keeps Cost.N / Cost.Filter as the sacrifice's and carries the
// discard on Cost.DiscardSideN / DiscardSideF. Skoa-heavy decks with Mountains
// put a second Skoa in hand beside a battlefield Skoa. The expected answer at
// every decision is gorge's.
var kskoaCostDecks = [2][]cardset.Entry{
	{
		{Name: "Skoa, Embermage", Count: 10}, {Name: "Divination", Count: 4}, {Name: "Shock", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 20},
	},
	{
		{Name: "Skoa, Embermage", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Divination", Count: 2}, {Name: "Mountain", Count: 18},
	},
}

// skoaCostAbility returns the table's Skoa activated ability's cost, failing
// when it is not the shape under test.
func skoaCostAbility(t *testing.T, tbl *kop.Table) *kop.Cost {
	t.Helper()
	for i := range tbl.Cards {
		abs := tbl.Cards[i].Abilities
		for j := range abs {
			c := &abs[j].Cost
			if abs[j].Kind == kop.AbActivated && c.Flags&(kop.CostSac|kop.CostDiscard) == kop.CostSac|kop.CostDiscard {
				if c.N != 2 || c.DiscardSideN != 1 || c.DiscardSideF == 0 || c.Flags&kop.CostDiscardFiltered != 0 {
					t.Fatalf("Skoa's cost is not Sac<2> beside a side Discard<1/named Skoa>: %+v", *c)
				}
				return c
			}
		}
	}
	t.Fatal("no activated ability with a sacrifice and a discard part in the pool")
	return nil
}

// TestSkoaCostLockstep plays the Skoa pool in lockstep.
func TestSkoaCostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "skoacost", kskoaCostDecks, 400, "KSKOACOST_SEEDS")
}

// TestSkoaCostMutations shows the pool exercises each part of the cost: with
// the discard's count raised to two, its filter dropped (any card pays), or
// the sacrifice's count lowered to one, some seed diverges.
func TestSkoaCostMutations(t *testing.T) {
	t.Parallel()
	for _, m := range []struct {
		name string
		fn   func(*kop.Cost)
	}{
		{"discard count two", func(c *kop.Cost) { c.DiscardSideN = 2 }},
		{"discard filter dropped", func(c *kop.Cost) { c.DiscardSideF = 0 }},
		{"sacrifice count one", func(c *kop.Cost) { c.N = 1 }},
	} {
		n := lockstepRun(t, "skoacost", kskoaCostDecks, 200, false, func(tbl *kop.Table) {
			cards := append([]kop.CardDef(nil), tbl.Cards...)
			for i := range cards {
				abs := append([]kop.Ability(nil), cards[i].Abilities...)
				for j := range abs {
					if abs[j].Kind == kop.AbActivated && abs[j].Cost.Flags&(kop.CostSac|kop.CostDiscard) == kop.CostSac|kop.CostDiscard {
						skoaCostAbility(t, &kop.Table{Cards: cards[i : i+1]}) // the precondition: the unmutated shape
						m.fn(&abs[j].Cost)
					}
				}
				cards[i].Abilities = abs
			}
			tbl.Cards = cards
		})
		if n == 0 {
			t.Errorf("%s: no seed diverged: the pool never activates Skoa's ability, or the kernel does not price this part", m.name)
		}
	}
}

// skoaCostPinnedSeed is a seed whose game activates Skoa several times.
const skoaCostPinnedSeed = 7

// TestSkoaCostResolves plays the pinned seed and requires Skoa's ability to be
// activated and resolved at least once, clean against gorge: a lockstep game
// alone would pass a pool whose Skoa never pays the cost.
func TestSkoaCostResolves(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "skoacost", kskoaCostDecks)
	tbl, _, _, err := set.Matchup(kskoaCostDecks[0], kskoaCostDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	skoaCostAbility(t, tbl)
	m, err := NewMatchup(reg, [2]string{"skoacost-a", "skoacost-b"}, gd, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, Config{})(m, skoaCostPinnedSeed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pushes, resolved := 0, 0
	rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40}
	rc.Finish = func(g *GorgeSide) {
		pending := false
		for _, e := range g.E.L.Events {
			if e.Kind == events.AbilityPush {
				if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Skoa, Embermage" {
					pushes++
					pending = true
				}
			}
			if e.Kind == events.Resolve && pending {
				resolved++
				pending = false
			}
		}
	}
	res := Lockstep(m, skoaCostPinnedSeed, k, castFirstPolicy(skoaCostPinnedSeed), rc)
	if res.Div != nil {
		t.Fatalf("diverged at decision %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if pushes == 0 || resolved == 0 {
		t.Fatalf("seed %d: Skoa's ability activated %d times, resolved %d: pin another seed", skoaCostPinnedSeed, pushes, resolved)
	}
}
