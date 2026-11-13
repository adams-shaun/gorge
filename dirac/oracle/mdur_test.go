package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MTGA durations lockstep: Duration$ UntilHostLeavesPlay
// over the generated mdur pool -- Extraction Specialist's returned creature
// can't attack or block while the Specialist stays (a host-bound override
// record with CantAttack), Gingerbrute can't be blocked except by haste
// creatures until end of turn or until it leaves (a host-bound emblem whose
// CantBlockBy reads it as the source). Shock and the trades remove the hosts
// mid-effect. The frozen FDN and BLB pair pools carry neither duration.

var kmdurDecks = [][2][]cardset.Entry{{
	{
		{Name: "Extraction Specialist", Count: 4}, {Name: "Gingerbrute", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Goblin Piker", Count: 4}, {Name: "Raging Goblin", Count: 3}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Giant Spider", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 13}, {Name: "Forest", Count: 6},
	},
}, {
	{
		{Name: "Gingerbrute", Count: 4}, {Name: "Coral Merfolk", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Extraction Specialist", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gray Ogre", Count: 3}, {Name: "Goblin Piker", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 5},
	},
}}

func TestMdurLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mdur")
	if err != nil {
		t.Fatal(err)
	}
	tal := &kmcombatTally{stacked: map[string]int{}, resolved: map[string]int{}}
	games := 0
	for mi, md := range kmdurDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 64; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kmdur-a", "kmdur-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castActivateKindPolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			tal.pending = map[state.ObjID]string{}
			tal.sengir = map[state.ObjID]int{}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tal.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if res.Stall != "" && strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	t.Logf("%d games; resolved %v", games, tal.resolved)
	for _, want := range []struct {
		key string
		min int
	}{
		{"Extraction Specialist/ChangeZone", 10},
		{"Gingerbrute/Effect", 20},
	} {
		if tal.resolved[want.key] < want.min {
			t.Errorf("%s resolved %d times, want at least %d", want.key, tal.resolved[want.key], want.min)
		}
	}
}
