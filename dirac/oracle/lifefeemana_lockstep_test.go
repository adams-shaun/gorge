package oracle

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The two mana-ability cost shapes the kernel's mana walk reads beyond a
// plain {T}: "{T}, Pay 1 life: Add {G} or {W}" (Horizon Canopy, Fiery Islet:
// a sole life ability with a colour choice, gorge's last-resort
// Consequence{Life: 1} per colour, kop.SoleLifeManaAb) and a fee-funded
// evaluated amount ("{2}, {T}: Add {B} for each Swamp you control", Cabal
// Coffers). The expected answer at every decision is gorge's.
var klifeFeeManaDecks = [2][]cardset.Entry{
	{
		{Name: "Horizon Canopy", Count: 5}, {Name: "Fiery Islet", Count: 3},
		{Name: "Cabal Coffers", Count: 4}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 2}, {Name: "Serra Angel", Count: 3},
		{Name: "Air Elemental", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Sengir Vampire", Count: 2}, {Name: "Gurmag Angler", Count: 1}, {Name: "Llanowar Elves", Count: 3},
	},
	{
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 6},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mana Leak", Count: 1},
	},
}

// tapCount counts the projections in which a battlefield permanent named by
// hash is tapped.
type tapCount struct {
	hash uint32
	n    atomic.Int64
}

// tappedSeen is a kernel side that counts them: a Horizon land or Cabal
// Coffers is tapped only by its mana ability (its other ability sacrifices
// it), so a count above zero proves the pool played the mechanism.
type tappedSeen struct {
	*realSide
	counts []*tapCount
}

func (s tappedSeen) Project(p *Projection) {
	s.realSide.Project(p)
	for _, tc := range s.counts {
		for i := 0; i < int(p.NInst); i++ {
			if c := &p.Cards[i]; c.NameHash == tc.hash && c.Zone == ZoneBattlefield && c.Flags&FTapped != 0 {
				tc.n.Add(1)
				break
			}
		}
	}
}

func TestLifeAndFeeManaLockstep(t *testing.T) {
	t.Parallel()
	seen := map[string]*tapCount{}
	var counts []*tapCount
	for _, n := range []string{"Horizon Canopy", "Fiery Islet", "Cabal Coffers"} {
		tc := &tapCount{hash: nameHash(n)}
		seen[n] = tc
		counts = append(counts, tc)
	}
	wrap := func(s Side) Side { return tappedSeen{s.(*realSide), counts} }
	if d := lockstepRunSide(t, "lifefeemana", klifeFeeManaDecks, 300, true, nil, wrap); d != 0 {
		t.Fatalf("%d divergent games", d)
	}
	for n, tc := range seen {
		if tc.n.Load() == 0 {
			t.Errorf("%s was never tapped for mana: the pool does not play the mechanism", n)
		}
		t.Logf("%s tapped in %d projections", n, tc.n.Load())
	}
}
