package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The token-attacking lockstep: Leonin Warleader's and Warren Warleader's
// (a charm mode of an AttackersDeclared trigger)
// TokenAttacking$ True mint and Adeline's
// RememberedPlayer one (a RepeatEach over the opponents), each token
// entering tapped and attacking the controller's opponent without being
// declared (no Raid Bombardment trigger, CR 508.4), then blocked and dealt
// combat damage like any attacker. No planeswalker is in the pool, so the
// CR 508.4 ask (StallTokAtkAsk) never arises. The frozen FDN and BLB pair
// pools carry no such mint.

var kmtokatkDecks = [][2][]cardset.Entry{{
	{
		{Name: "Leonin Warleader", Count: 4}, {Name: "Adeline, Resplendent Cathar", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Raid Bombardment", Count: 3}, {Name: "Gray Ogre", Count: 1}, {Name: "Warren Warleader", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Plains", Count: 14}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Coral Merfolk", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 5},
	},
}, {
	{
		{Name: "Leonin Warleader", Count: 4}, {Name: "Coral Merfolk", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Giant Spider", Count: 3}, {Name: "Savannah Lions", Count: 4}, {Name: "Raid Bombardment", Count: 2},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Adeline, Resplendent Cathar", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Gray Ogre", Count: 2}, {Name: "Warren Warleader", Count: 2},
		{Name: "Hill Giant", Count: 4}, {Name: "Coral Merfolk", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 6},
	},
}}

// kmtokatkTally counts, on the gorge side, the tokens that entered
// attacking (a token first seen on the battlefield already attacking) and
// those of them later dealt combat damage or blocked.
type kmtokatkTally struct {
	kmcombatTally
	seen     map[state.ObjID]bool
	entered  map[string]int
	blocked  int
	blockedS map[state.ObjID]bool
}

func (t *kmtokatkTally) check(ref *GorgeSide, k Side) *Divergence {
	t.kmcombatTally.check(ref, k)
	g := ref.E.G
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || !o.IsToken || o.Face() == nil {
				continue
			}
			if !t.seen[id] {
				t.seen[id] = true
				if o.IsAttacking {
					t.entered[o.Face().Name]++
				}
			}
			if o.IsAttacking && len(o.BlockedBy) > 0 && !t.blockedS[id] {
				t.blockedS[id] = true
				t.blocked++
			}
		}
	}
	return nil
}

func TestMtokatkLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mtokatk")
	if err != nil {
		t.Fatal(err)
	}
	tal := &kmtokatkTally{kmcombatTally: kmcombatTally{stacked: map[string]int{}, resolved: map[string]int{}}, entered: map[string]int{}}
	games := 0
	for mi, md := range kmtokatkDecks {
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
			m, err := NewMatchup(reg, [2]string{"kmtokatk-a", "kmtokatk-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := kmcombatPolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			tal.pending = map[state.ObjID]string{}
			tal.sengir = map[state.ObjID]int{}
			tal.seen = map[state.ObjID]bool{}
			tal.blockedS = map[state.ObjID]bool{}
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
	t.Logf("%d games; tokens entered attacking %v; blocked %d; stacked %v", games, tal.entered, tal.blocked, tal.stacked)
	for _, name := range []string{"Cat Token", "Human Token", "Rabbit Token"} {
		if tal.entered[name] < 5 {
			t.Errorf("%s entered attacking %d times, want at least 5", name, tal.entered[name])
		}
	}
	if tal.blocked < 3 {
		t.Errorf("%d attacking tokens were blocked, want at least 3", tal.blocked)
	}
}
