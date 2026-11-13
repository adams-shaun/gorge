package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MTGA combat-props lockstep: constructed matchups
// over the generated mcombat pool, so the expected answer at every
// decision is gorge's. The frozen FDN and BLB pair pools carry none of the
// combat-state filter words this pool lifts: Sengir Vampire's DamagedBy
// (the source-mark records), Tinder Wall's blockedBySourceLKI
// (a target chosen while it blocks, read again after its sacrifice),
// Subterranean Schooner's CrewedThisTurn (the crew marks) and Raid
// Bombardment's TriggeredDefender (OpAtkPWGuard: no planeswalker is in the
// pool, so it never stalls).

var kmcombatDecks = [][2][]cardset.Entry{{
	{
		{Name: "Sengir Vampire", Count: 4}, {Name: "Tinder Wall", Count: 4}, {Name: "Prey Upon", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 2}, {Name: "Walking Corpse", Count: 3},
		{Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Raid Bombardment", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Gray Ogre", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 14}, {Name: "Plains", Count: 5},
	},
}, {
	{
		{Name: "Subterranean Schooner", Count: 4}, {Name: "Coral Merfolk", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Sengir Vampire", Count: 3}, {Name: "Wall of Wood", Count: 2}, {Name: "Savannah Lions", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Tinder Wall", Count: 4}, {Name: "Raid Bombardment", Count: 3},
		{Name: "Gray Ogre", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Prey Upon", Count: 2},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 7},
	},
}}

// kmcombatTally counts, on the gorge side, the lifted mechanics: each
// carrier ability put on the stack (source name / API: the filter matched
// or the condition held there) and each one that left it, and the +1/+1
// counters Sengir Vampire's trigger put on it.
type kmcombatTally struct {
	pending  map[state.ObjID]string
	stacked  map[string]int
	resolved map[string]int
	sengir   map[state.ObjID]int
	counters int
}

func (t *kmcombatTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	on := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || o.Ability == nil {
			continue
		}
		on[id] = true
		if _, ok := t.pending[id]; ok {
			continue
		}
		name := ""
		if src := g.Obj(o.Source); src != nil && src.Face() != nil {
			name = src.Face().Name
		}
		key := name + "/" + o.Ability.API
		t.pending[id] = key
		t.stacked[key]++
	}
	for id, key := range t.pending {
		if !on[id] {
			t.resolved[key]++
			delete(t.pending, id)
		}
	}
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil || o.Face().Name != "Sengir Vampire" {
				continue
			}
			n := 0
			for _, c := range o.Counters {
				if c.Kind == "P1P1" {
					n += int(c.N)
				}
			}
			if n > t.sengir[id] {
				t.counters += n - t.sengir[id]
			}
			t.sengir[id] = n
		}
	}
	return nil
}

func TestMcombatLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mcombat")
	if err != nil {
		t.Fatal(err)
	}
	tal := &kmcombatTally{pending: map[state.ObjID]string{}, stacked: map[string]int{}, resolved: map[string]int{}}
	games := 0
	for mi, md := range kmcombatDecks {
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
		for seed := uint64(1); seed <= 96; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kmcombat-a", "kmcombat-b"}, decks, 1)
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
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if res.Stall != "" && strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	t.Logf("%d games; stacked %v; resolved %v; Sengir counters %d", games, tal.stacked, tal.resolved, tal.counters)
	for _, want := range []struct {
		key string
		min int
	}{
		{"Sengir Vampire/PutCounter", 3},
		{"Tinder Wall/DealDamage", 2},
		{"Subterranean Schooner/Explore", 3},
		{"Raid Bombardment/DealDamage", 5},
	} {
		if tal.resolved[want.key] < want.min {
			t.Errorf("%s resolved %d times, want at least %d", want.key, tal.resolved[want.key], want.min)
		}
	}
	if tal.counters < 3 {
		t.Errorf("Sengir Vampire got %d +1/+1 counters, want at least 3", tal.counters)
	}
}
