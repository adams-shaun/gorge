package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A reduction that waits for the target leaves gorge's planner pricing the
// full cost. When the sources its census can see do not pay that, the cast is
// not one of its plays until the pool pays it: neither Hidden Grotto's filter
// nor Imprisoned in the Moon's granted {C} makes it one.
func TestTargetDependentCostOfferLockstep(t *testing.T) {
	t.Parallel()
	reg, _ := kcastSetup(t)
	for _, c := range []struct {
		name  string
		set   string
		decks [2][]cardset.Entry
	}{
		{"filter", "blb", [2][]cardset.Entry{
			{{Name: "Hidden Grotto", Count: 6}, {Name: "Mountain", Count: 18}, {Name: "Dire Downdraft", Count: 16}},
			{{Name: "Plains", Count: 18}, {Name: "Brightblade Stoat", Count: 22}},
		}},
		{"granted", "fdn", [2][]cardset.Entry{
			{{Name: "Island", Count: 22}, {Name: "Imprisoned in the Moon", Count: 18}},
			{{Name: "Plains", Count: 18}, {Name: "Luminous Rebuke", Count: 8}, {Name: "Savannah Lions", Count: 14}},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			set, err := cardset.ByName(c.set)
			if err != nil {
				t.Fatal(err)
			}
			decks := kcastDeck(t, reg, c.decks)
			tbl, _, _, err := set.Matchup(c.decks[0], c.decks[1])
			if err != nil {
				t.Fatal(err)
			}
			var cfg Config
			for seed := uint64(1); seed <= 8; seed++ {
				m, err := NewMatchup(reg, [2]string{c.name + "-a", c.name + "-b"}, decks, 0)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30})
				CloseSide(k)
				if res.Div != nil {
					t.Errorf("seed %d: diverged at %d: %s %s %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail)
				}
			}
		})
	}
}
