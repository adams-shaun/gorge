package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Pump Duration$ UntilYourNextUpkeep lockstep (CR 611.2b): Erhnam
// Djinn's upkeep trigger grants forestwalk to an opponent's non-Wall creature
// until the resolving controller's next upkeep begins. The grant is live
// through the controller's untap step and ends as their upkeep begins (gorge
// dropUntilYourNextUpkeep); the fixture pool is dirac/cardset/pools/fixtures/
// pumpupkeep.
const pumpUpkeepPool = "pumpupkeep"

var pumpUpkeepDecks = [2][]cardset.Entry{
	{
		{Name: "Erhnam Djinn", Count: 8}, {Name: "Grizzly Bears", Count: 8}, {Name: "Forest", Count: 24},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 8}, {Name: "Forest", Count: 24},
	},
}

// pumpUpkeepPolicy casts Erhnam Djinn whenever it is offered (the grant is
// the card's upkeep trigger) and otherwise uses castFirstPolicy. casts counts
// the Djinn casts so the test can prove the pool reaches the grant.
func pumpUpkeepPolicy(seed uint64, casts *int) Policy {
	base := castFirstPolicy(seed)
	var pj Projection
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return base.Pick(c)
		}
		c.G.Project(&pj)
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			src := int(a.Src())
			if src < int(pj.NInst) && src < MaxInst && pj.Cards[src].NameHash == nameHash("Erhnam Djinn") {
				*casts++
				return a
			}
		}
		return base.Pick(c)
	})
}

// TestPumpUpkeepLockstep: the Djinn's grant is carried in lockstep. The test
// first checks the lowering carries the new duration and the class on the
// Override row, then plays games and requires the Djinn to be cast.
func TestPumpUpkeepLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, pumpUpkeepPool, pumpUpkeepDecks)
	if !SetDeckSupported(set, gd[0]) {
		t.Fatal("Erhnam Djinn's deck is not playable in the pumpupkeep pool")
	}
	tbl, _, _, err := set.Matchup(pumpUpkeepDecks[0], pumpUpkeepDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range tbl.Cards {
		if tbl.Cards[i].Name != "Erhnam Djinn" {
			continue
		}
		for j := range tbl.Cards[i].Abilities {
			for _, in := range tbl.Cards[i].Abilities[j].Prog {
				if in.Op != kop.OpOverride || in.A.K != kop.VConst || in.A.N != kop.DurUntilYourNextUpkeep {
					continue
				}
				if int(in.C) >= len(tbl.Overrides) || !tbl.Overrides[in.C].UntilUpkeep {
					t.Fatalf("Erhnam Djinn's override %d does not carry UntilUpkeep", in.C)
				}
				found = true
			}
		}
	}
	if !found {
		t.Fatal("Erhnam Djinn did not lower to DurUntilYourNextUpkeep")
	}
	var casts int
	clean, _, divs := lockstepSeedsMode(t, reg, set, pumpUpkeepPool, pumpUpkeepDecks, gd, 1, 48, lockstepFailEach, nil, nil,
		func(seed uint64) Policy { return pumpUpkeepPolicy(seed, &casts) })
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if casts == 0 {
		t.Fatal("the lockstep never cast Erhnam Djinn: the pool does not reach the grant")
	}
	t.Logf("Erhnam Djinn cast %d times; %d clean games", casts, clean)
}
