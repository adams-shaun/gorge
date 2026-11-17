package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// DB$ Clash's lockstep (dirac/kgen/lower_clash.go, kernel act_clash.go):
// Adder-Staff Boggart and Bog Hoodlums (a counter on a win), Fire Juggler
// (4 damage to its blockers on a win) and Broken Ambitions (the loser's
// spell's controller mills on a win) each clash with the opponent. Gorge asks
// the controller's top-or-bottom pick and then the opponent's; the two engines
// must agree on both asks, the verdict and the library order, decision by
// decision. The decks carry creatures of mana values 1, 2 and 4 so clashes are
// won, lost and tied.
func kclashDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Adder-Staff Boggart", Count: 4}, {Name: "Bog Hoodlums", Count: 4}, {Name: "Fire Juggler", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Savannah Lions", Count: 2},
			{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 9},
		},
		{
			{Name: "Broken Ambitions", Count: 4}, {Name: "Fire Juggler", Count: 3}, {Name: "Adder-Staff Boggart", Count: 3},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 4}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Island", Count: 11}, {Name: "Mountain", Count: 10},
		},
	}
}

// kclashPolicy plays castFirstPolicy and counts, from gorge's options, the
// top-or-bottom asks it is posed.
func kclashPolicy(asks *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && !counted[c.Seq] {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && o.Kind == "top" {
						*asks++
						counted[c.Seq] = true
						break
					}
				}
			}
			return base.Pick(c)
		})
	}
}

func TestClashLockstep(t *testing.T) {
	t.Parallel()
	asks := new(int)
	lockstepRunPolicy(t, "clash", kclashDecks(), 60, true, nil, nil, kclashPolicy(asks))
	if *asks == 0 {
		t.Fatal("no clash placement ask was posed in the games; the lockstep proves nothing")
	}
	t.Logf("clash placement asks: %d", *asks)
}
