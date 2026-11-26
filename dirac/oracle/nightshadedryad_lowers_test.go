package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The colourless-then-any-colour pair lockstep: Nightshade Dryad's "{T}: Add
// {C}" beside its "{T}: Add one mana of any color". gorge's census unit holds
// the {C} alternative then one per colour; the kernel's is the union C|WUBRG
// (mana_multicolour.go prepMultiMana), Mirrex's gate-on view with no gate.
// Both decks lean on the Dryad for their colours; Cackling Prowler's ward
// runs gorge's mana window, which asks the Dryad's ability.
var kdryadDecks = [2][]cardset.Entry{
	{
		{Name: "Nightshade Dryad", Count: 10}, {Name: "Wastes", Count: 6}, {Name: "Forest", Count: 4},
		{Name: "Savannah Lions", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Cackling Prowler", Count: 4}, {Name: "Memnite", Count: 4},
	},
	{
		{Name: "Nightshade Dryad", Count: 10}, {Name: "Wastes", Count: 5}, {Name: "Mountain", Count: 3},
		{Name: "Island", Count: 2}, {Name: "Plains", Count: 2}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Cackling Prowler", Count: 3},
	},
}

// kdryadTally counts, on gorge's side, Nightshade Dryad's mana activations by
// ability.
type kdryadTally struct{ any, colourless int }

// kdryadRun plays seeds 1..n with pol, failing on each divergence or
// unmodelled stall (unless mutated: the first divergence is returned instead).
func kdryadRun(t *testing.T, pol func(uint64) Policy, n uint64, tal *kdryadTally, mutated bool) *Divergence {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("nightshadedryad")
	if err != nil {
		t.Fatal(err)
	}
	kd := kdryadDecks
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
		m, err := NewMatchup(reg, [2]string{"nd-a", "nd-b"}, decks, 1)
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
			if ref.E.ManaAbilityHook == nil && tal != nil {
				e := ref.E
				e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
					if o := e.G.Obj(src); o == nil || o.Face() == nil || o.Face().Name != "Nightshade Dryad" {
						return
					}
					if sa.Params["Produced"] == "Any" {
						tal.any++
					} else {
						tal.colourless++
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

// TestNightshadeDryadLowers: random and explore policies, no divergence and no
// unmodelled stall, and gorge's side activates both of the Dryad's abilities,
// so the lockstep compares the union unit's {C} and its colours.
func TestNightshadeDryadLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal kdryadTally
	kdryadRun(t, pkPolicy, 48, &tal, false)
	kdryadRun(t, ExplorePolicy, 64, &tal, false)
	t.Logf("Nightshade Dryad on gorge's side: any colour %d, {C} %d", tal.any, tal.colourless)
	if tal.any == 0 || tal.colourless == 0 {
		t.Errorf("Nightshade Dryad activations: any colour %d, {C} %d; want both", tal.any, tal.colourless)
	}
}

// TestNightshadeDryadMutation: with the pair read as its {C} ability alone
// (dirac.DebugColourlessAny) some seed diverges, so TestNightshadeDryadLowers
// depends on the kernel's union unit and does not pass vacuously.
func TestNightshadeDryadMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugColourlessAny = true
	defer func() { dirac.DebugColourlessAny = false }()
	if kdryadRun(t, pkPolicy, 48, nil, true) == nil && kdryadRun(t, ExplorePolicy, 64, nil, true) == nil {
		t.Error("no divergence with the pair read as {C} alone")
	}
}
