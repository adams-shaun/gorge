package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Renown lockstep (CR 702.112): constructed matchups over the generated
// renown pool. K:Renown's combat-damage trigger puts its counters and sets the
// renowned designation; Consul's Lieutenant's attack trigger, Scab-Clan
// Berserker's spell-cast trigger and Goblin Glory Chaser's and Honored
// Hierarch's statics then read IsRenowned, and Shock kills a renowned
// creature. The frozen FDN and BLB pair pools carry none of these.

var renownDecks = [2][]cardset.Entry{
	{
		{Name: "Consul's Lieutenant", Count: 4}, {Name: "Topan Freeblade", Count: 4},
		{Name: "Goblin Glory Chaser", Count: 4}, {Name: "Scab-Clan Berserker", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Honored Hierarch", Count: 4}, {Name: "Undercity Troll", Count: 4},
		{Name: "Rhox Maulers", Count: 4}, {Name: "Outland Colossus", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 8},
	},
}

// renownWatch wraps the eager policy with a count, per card name, of the
// battlefield permanents gorge holds renowned, and of the renowned
// Consul's Lieutenants that are attacking: the precondition the lockstep's
// agreement depends on (the designation was set, and its attack trigger had
// a renowned attacker to read it).
func renownWatch(seed uint64, played, renowned, renownedAttack map[string]int) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
					ob := c.G.E.G.Obj(id)
					if ob == nil || !ob.Renowned || ob.Face() == nil {
						continue
					}
					renowned[ob.Face().Name]++
					if ob.IsAttacking {
						renownedAttack[ob.Face().Name]++
					}
				}
			}
		}
		return inner.Pick(c)
	})
}

func TestRenownLowers(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("renown")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range renownDecks {
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
	tbl, _, _, err := set.Matchup(renownDecks[0], renownDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	played := map[string]int{}
	renowned := map[string]int{}
	renownedAttack := map[string]int{}
	games := 0
	for seed := uint64(1); seed <= 512; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"renown-a", "renown-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := renownWatch(seed, played, renowned, renownedAttack)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, plays %v, renowned %v, renowned attackers %v", games, played, renowned, renownedAttack)
	for _, n := range []string{"Consul's Lieutenant", "Topan Freeblade", "Goblin Glory Chaser", "Scab-Clan Berserker", "Honored Hierarch", "Undercity Troll", "Rhox Maulers", "Outland Colossus"} {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
	for _, n := range []string{"Consul's Lieutenant", "Topan Freeblade", "Goblin Glory Chaser", "Honored Hierarch"} {
		if renowned[n] == 0 {
			t.Errorf("%s was never renowned: the designation was not exercised", n)
		}
	}
	if renownedAttack["Consul's Lieutenant"] == 0 {
		t.Errorf("no renowned Consul's Lieutenant ever attacked: its IsRenowned attack trigger was not exercised")
	}
}
