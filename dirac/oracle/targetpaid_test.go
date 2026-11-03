package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A cast whose only legal target is the Treasure its own payment sacrifices
// (CR 601.2c targets, 601.2g mana, 608.2b illegal target) is offered by gorge
// whatever the spell's shape; the kernel must offer and keep it too. Into the
// Roil (kicker: an optional cost) and Temporal Cleansing (convoke: a credit)
// are shapes gorge's plain planner declines; beside a lone Treasure from
// Strike It Rich and an opponent with lands only, the Treasure's {U} is the only
// way to pay and the only nonland permanent to target.
func TestOnlyTargetPaidCastLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, spell := range []string{"Into the Roil", "Temporal Cleansing"} {
		t.Run(spell, func(t *testing.T) { lockstepTargetPaid(t, reg, spell) })
	}
}

func lockstepTargetPaid(t *testing.T, reg *cards.Registry, spell string) {
	set, err := cardset.RawByName("targetpaid")
	if err != nil {
		t.Fatal(err)
	}
	mine := []cardset.Entry{{Name: "Strike It Rich", Count: 10}, {Name: spell, Count: 10}, {Name: "Mountain", Count: 14}}
	theirs := []cardset.Entry{{Name: "Mountain", Count: 40}}
	deck := func(es []cardset.Entry) []*cards.Card {
		var out []*cards.Card
		for _, e := range es {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				out = append(out, cd)
			}
		}
		return out
	}
	tbl, _, _, err := set.Matchup(mine, theirs)
	if err != nil {
		t.Fatal(err)
	}
	stood := map[string]int{}
	rank := map[string]int{spell: 3, "Strike It Rich": 1}
	for seed := uint64(1); seed <= 40; seed++ {
		m, err := NewMatchup(reg, [2]string{"paid-a", "paid-b"}, [2][]*cards.Card{deck(mine), deck(theirs)}, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := RandomPolicy(seed)
		seen := map[string]bool{}
		pol := PolicyFunc(func(c *PickCtx) Canon {
			for _, id := range c.G.E.G.Stack {
				if n := chName(c.G.E.G, id); n == spell && !seen[n] {
					seen[n] = true
					stood[n]++
				}
			}
			best, bestRank := Canon(0), 0
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if id, ok := kroleRefObj(c.G, a.Src()); ok {
					if r := rank[chName(c.G.E.G, id)]; r > bestRank {
						best, bestRank = a, r
					}
				}
			}
			if bestRank > 0 {
				return best
			}
			return base.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 100})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("seed %d: diverged at %d: %+v", seed, res.Div.At, *res.Div)
		}
	}
	t.Logf("games where each spell stood on the stack: %v", stood)
	if stood[spell] == 0 {
		t.Errorf("stood %v: want both spells cast", stood)
	}
}
