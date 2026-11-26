package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The energy mana lockstep: Aether Hub's and Solar Transformer's second
// ability, "{T}, Pay {E}: Add one mana of any color", beside their plain
// "{T}: Add {C}", is the energy form of the card's counter gap (kernel
// mana_countercost.go, kop.CtrTapSecondManaAb): gorge's census leaves it a
// relaxable once-only gap its prefix search activates, payable while the
// controller holds the energy (pay.EnergyPayable), the activation charging it
// (pay.ChargeEnergyCost). Both decks run on Wastes and the two energy sources
// with a few basics, so a coloured spell is often funded by the energy
// ability alone, and the energy the enter triggers give runs out.
var kenergyDecks = [2][]cardset.Entry{
	{
		{Name: "Aether Hub", Count: 6}, {Name: "Solar Transformer", Count: 4}, {Name: "Wastes", Count: 5},
		{Name: "Mountain", Count: 1}, {Name: "Plains", Count: 1},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Serra Angel", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Memnite", Count: 4}, {Name: "Doom Blade", Count: 4},
	},
	{
		{Name: "Aether Hub", Count: 4}, {Name: "Solar Transformer", Count: 4}, {Name: "Wastes", Count: 6},
		{Name: "Swamp", Count: 2}, {Name: "Forest", Count: 1},
		{Name: "Doom Blade", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Memnite", Count: 4}, {Name: "Hill Giant", Count: 4},
	},
}

var kenergySources = map[string]bool{"Aether Hub": true, "Solar Transformer": true}

// kenergyRun plays seeds with pol, failing on each divergence or unmodelled
// stall (unless mutated: the first divergence is returned instead). byName
// counts, from gorge's side, each source's energy mana activations.
func kenergyRun(t *testing.T, pol func(uint64) Policy, n uint64, byName map[string]int, mutated bool) *Divergence {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("energymana")
	if err != nil {
		t.Fatal(err)
	}
	kd := kenergyDecks
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
		m, err := NewMatchup(reg, [2]string{"em-a", "em-b"}, decks, 1)
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
					if sa.API != "Mana" || !strings.Contains(sa.ParamStr(cards.PKCost), "PayEnergy") {
						return
					}
					if o := e.G.Obj(src); o != nil && o.Face() != nil && kenergySources[o.Face().Name] {
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
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return nil
}

// TestEnergyManaLowersRandom: the random policy's 48 seeds, no divergence and
// no unmodelled stall. (Split from TestEnergyManaLowers for the 60 s test budget.)
func TestEnergyManaLowersRandom(t *testing.T) {
	if isolate(t) {
		return
	}
	kenergyRun(t, pkPolicy, 48, nil, false)
}

// TestEnergyManaLowers: the explore policy, no divergence and no unmodelled
// stall, and the run pays energy for mana on gorge's side with both sources,
// so the lockstep compares the energy ability's walk.
func TestEnergyManaLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	byName := map[string]int{}
	kenergyRun(t, ExplorePolicy, 64, byName, false)
	for name := range kenergySources {
		t.Logf("%s energy ability activated %d times on gorge's side", name, byName[name])
		if byName[name] == 0 {
			t.Errorf("%s's energy mana ability was never activated on gorge's side", name)
		}
	}
}

// TestEnergyManaMutation: with the counter-cost gaps hidden
// (dirac.DebugNoCtrMana) the explore run diverges, so TestEnergyManaLowers
// depends on the kernel playing the energy ability and does not pass
// vacuously.
func TestEnergyManaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCtrMana = true
	defer func() { dirac.DebugNoCtrMana = false }()
	if kenergyRun(t, ExplorePolicy, 64, nil, true) == nil {
		t.Error("no divergence without the energy gap")
	}
}
