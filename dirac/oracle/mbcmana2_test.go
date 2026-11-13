package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC mana-abilities-2 lockstep: Relic of Legends' second "tap an untapped
// legendary creature you control: add one mana of any color" beside its {T}
// ability, Krark-Clan Ironworks' "sacrifice an artifact: add {C}{C}" and
// Apprentice Wizard's "{U}, {T}: add {C}{C}{C}". The first deck holds all
// three over artifacts, legendary creatures and spells only their mana casts
// (Air Elemental, Serra Angel, Juggernaut); the second is a plain creature
// deck. Every tracked card's mana ability must be activated on gorge's side,
// so the lockstep compares the kernel's walk of it.
var kmbc2Names = []string{"Apprentice Wizard", "Relic of Legends"}

// kmbc2Cost: the cost text of the tracked mana ability, so the count is of that
// ability's activations and not of Relic of Legends' own {T} one.
var kmbc2Cost = map[string]string{"Relic of Legends": "tapXType", "Krark-Clan Ironworks": "Sac<", "Apprentice Wizard": "T"}

var kmbc2Opp = []cardset.Entry{
	{Name: "Hill Giant", Count: 2}, {Name: "Wind Drake", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Doom Blade", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Serra Angel", Count: 2},
	{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 10},
}

// kmbc2Decks: one deck per spelling, the card in numbers over the spells its
// mana casts.
var kmbc2Decks = map[string][]cardset.Entry{
	"Relic of Legends": {
		{Name: "Relic of Legends", Count: 5}, {Name: "Isamaru, Hound of Konda", Count: 5}, {Name: "Mind Stone", Count: 2},
		{Name: "Juggernaut", Count: 3}, {Name: "Air Elemental", Count: 4}, {Name: "Serra Angel", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Plains", Count: 9},
	},
	"Krark-Clan Ironworks": {
		{Name: "Krark-Clan Ironworks", Count: 5}, {Name: "Ornithopter", Count: 4}, {Name: "Mind Stone", Count: 3},
		{Name: "Sol Ring", Count: 1}, {Name: "Juggernaut", Count: 4}, {Name: "Air Elemental", Count: 4},
		{Name: "Serra Angel", Count: 4}, {Name: "Island", Count: 8}, {Name: "Plains", Count: 7},
	},
	"Apprentice Wizard": {
		{Name: "Apprentice Wizard", Count: 6}, {Name: "Sol Ring", Count: 1}, {Name: "Juggernaut", Count: 4},
		{Name: "Air Elemental", Count: 4}, {Name: "Serra Angel", Count: 3}, {Name: "Mind Stone", Count: 2},
		{Name: "Island", Count: 11}, {Name: "Plains", Count: 9},
	},
}

// kmbc2Run plays seeds with pol, failing on each divergence or unmodelled
// stall (unless mutated: the first divergence is returned instead). byName
// counts, from gorge's side, each tracked card's mana ability activations.
func kmbc2Run(t *testing.T, kd [2][]cardset.Entry, tracked string, pol func(uint64) Policy, n uint64, byName map[string]int, mutated bool) *Divergence {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcmana2")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kd {
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
	tbl, _, _, err := set.Matchup(kd[0], kd[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mm2-a", "mm2-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			if ref.E.ManaAbilityHook == nil && byName != nil {
				e := ref.E
				e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
					if !strings.Contains(sa.ParamStr(cards.PKCost), kmbc2Cost[tracked]) {
						return
					}
					if o := e.G.Obj(src); o != nil && o.Face() != nil && o.Face().Name == tracked {
						byName[o.Face().Name]++
					}
				}
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20, Check: check})
		CloseSide(k)
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil && mutated {
			return d
		} else if d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Logf("gorge pool %v", g.E.G.Players[0].Pool)
			for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
				if o := g.E.G.Obj(id); o != nil {
					t.Logf("battlefield %d: %q tapped=%v", id, o.Face().Name, o.Tapped)
				}
			}
			for _, id := range g.E.G.Zone(state.ZHand, 0) {
				if o := g.E.G.Obj(id); o != nil {
					t.Logf("hand %d: %q", id, o.Face().Name)
				}
			}
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return nil
}

// TestMBCManaAbilities2Lowers: per spelling, random and explore policies, no
// divergence and no unmodelled stall, and the explore run activates the card's
// mana ability on gorge's side so the lockstep actually compares the kernel's
// walk of it. Without the lowering the card is refused and the matchup fails
// to build.
func TestMBCManaAbilities2Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, name := range kmbc2Names {
		name := name
		t.Run(strings.ReplaceAll(name, " ", "_"), func(t *testing.T) { kmbc2Spelling(t, name) })
	}
}

func kmbc2Spelling(t *testing.T, name string) {
	{
		kd := [2][]cardset.Entry{kmbc2Decks[name], kmbc2Opp}
		kmbc2Run(t, kd, name, pkPolicy, 48, nil, false)
		byName := map[string]int{}
		kmbc2Run(t, kd, name, ExplorePolicy, 64, byName, false)
		t.Logf("%s mana ability activated %d times on gorge's side", name, byName[name])
		if byName[name] == 0 {
			t.Errorf("%s's mana ability was never activated on gorge's side", name)
		}
	}
}
