package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Kira Nerys, Firebrand lockstep: "if a creature you would put onto the
// battlefield while you control an attacking creature, it enters tapped and
// attacking instead" (CR 614.12, 508.4: never declared, so no Attacks trigger).
// The pool holds Kira beside Raise the Alarm (two tokens) and Ambush Viper
// (a creature spell), both at instant speed during combat, Cloudshift (a
// flickered creature re-enters), Leonin Warleader (its attack mints attacking
// tokens too) and vanilla creatures, so the replacement is met entering as a
// spell, a token and a blink, with and without an attacker. No planeswalker or battle is in the pool, so the
// two-defender stall (StallTokAtkAsk) never arises.
var kiraDecks = [][2][]cardset.Entry{{
	{
		{Name: "Kira Nerys, Firebrand", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Cloudshift", Count: 2},
		{Name: "Ambush Viper", Count: 4}, {Name: "Leonin Warleader", Count: 3}, {Name: "Savannah Lions", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Coral Merfolk", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 5},
	},
}, {
	{
		{Name: "Kira Nerys, Firebrand", Count: 4}, {Name: "Raise the Alarm", Count: 4}, {Name: "Ambush Viper", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Gray Ogre", Count: 3}, {Name: "Leonin Warleader", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Kira Nerys, Firebrand", Count: 3}, {Name: "Raise the Alarm", Count: 4}, {Name: "Ambush Viper", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 4},
	},
}}

// kiraTally counts, on the gorge side, each permanent first seen on the
// battlefield already tapped and attacking (it was never declared), by name,
// and Kira herself among them.
type kiraTally struct {
	seen    map[state.ObjID]bool
	entered map[string]int
}

func (t *kiraTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || t.seen[id] {
				continue
			}
			t.seen[id] = true
			if o.IsAttacking && o.Tapped {
				t.entered[o.Face().Name]++
			}
		}
	}
	return nil
}

// kiraRun plays the pool's games and returns the divergence count and the
// tally. A mutation run logs the first divergence and stops; a real run fails
// on every one.
func kiraRun(t *testing.T, seeds uint64, mutation bool) (int, *kiraTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("kira")
	if err != nil {
		t.Fatal(err)
	}
	tal := &kiraTally{entered: map[string]int{}}
	divs := 0
	for mi, md := range kiraDecks {
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
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kira-a", "kira-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castActivateKindPolicy(seed)
			if seed%2 == 0 {
				pol = RandomPolicy(seed)
			}
			tal.seen = map[state.ObjID]bool{}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tal.check})
			CloseSide(k)
			if d := res.Div; d != nil {
				divs++
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				if mutation {
					t.Logf("matchup %d seed %d diverged at decision %d: %s %s", mi, seed, d.At, d.What, d.Field)
					return divs, tal
				}
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if res.Stall != "" && strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	return divs, tal
}

func TestKiraLockstep(t *testing.T) {
	t.Parallel()
	_, tal := kiraRun(t, 256, false)
	t.Logf("entered tapped and attacking: %v", tal.entered)
	for _, name := range []string{"Soldier Token", "Ambush Viper"} {
		if tal.entered[name] < 5 {
			t.Errorf("%s entered tapped and attacking %d times, want at least 5", name, tal.entered[name])
		}
	}
}

// TestKiraMutation: each mutation of the lifted replacement must diverge.
// (1) enters tapped but not attacking; (2) the IsPresent gate dropped.
func TestKiraMutation(t *testing.T) {
	for _, mode := range []uint8{1, 2} {
		func() {
			dirac.DebugKiraMutate = mode
			defer func() { dirac.DebugKiraMutate = 0 }()
			if n, _ := kiraRun(t, 256, true); n == 0 {
				t.Errorf("mutation %d diverged in no game", mode)
			}
		}()
	}
}
