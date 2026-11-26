package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The pain-first coloured pair lockstep: Murmuring Bosk's "{T}: Add {W} or
// {B}. Murmuring Bosk deals 1 damage to you" beside the {G} its Forest type
// gives it. gorge's planner prices the {G} as the normal alternative and the
// painful choice as a last resort (pay.PaymentPlanDamageRider); the kernel
// reads the painland pair with its ability indices swapped (mana_pain.go
// prepPainFirst). Both decks lean on the Bosk for white and black; Ironroot
// Treefolk answers its enters-tapped reveal; Cackling Prowler's ward runs
// gorge's mana window, which asks the Bosk's ability.
var kboskDecks = [2][]cardset.Entry{
	{
		{Name: "Murmuring Bosk", Count: 10}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 2},
		{Name: "Ironroot Treefolk", Count: 3}, {Name: "Savannah Lions", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Cackling Prowler", Count: 4}, {Name: "Memnite", Count: 4},
	},
	{
		{Name: "Murmuring Bosk", Count: 10}, {Name: "Forest", Count: 4}, {Name: "Swamp", Count: 3},
		{Name: "Ironroot Treefolk", Count: 3}, {Name: "Typhoid Rats", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Cackling Prowler", Count: 4},
	},
}

// kboskTally counts, on gorge's side, Murmuring Bosk's mana activations by
// ability.
type kboskTally struct{ pain, green int }

// kboskRun plays seeds lo..hi with pol, failing on each divergence or
// unmodelled stall (unless mutated: the first divergence is returned instead).
func kboskRun(t *testing.T, pol func(uint64) Policy, lo, hi uint64, tal *kboskTally, mutated bool) *Divergence {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("murmuringbosk")
	if err != nil {
		t.Fatal(err)
	}
	kd := kboskDecks
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
	for seed := lo; seed <= hi; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mb-a", "mb-b"}, decks, 1)
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
					if o := e.G.Obj(src); o == nil || o.Face() == nil || o.Face().Name != "Murmuring Bosk" {
						return
					}
					if sa.Params["Produced"] == "G" {
						tal.green++
					} else {
						tal.pain++
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
			from := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[from:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return nil
}

// TestMurmuringBoskLowers: the random policy over seeds 1-80, no divergence
// and no unmodelled stall, and gorge's side activates both of the Bosk's
// abilities, so the lockstep compares the pair's {G} and its painful colours.
func TestMurmuringBoskLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	kboskBoth(t, pkPolicy)
}

// TestMurmuringBoskLowersExplore is TestMurmuringBoskLowers under the
// explore policy (split to keep each test inside the 60 s budget).
func TestMurmuringBoskLowersExplore(t *testing.T) {
	if isolate(t) {
		return
	}
	kboskBoth(t, ExplorePolicy)
}

// kboskBoth plays seeds 1-80 with pol and asks for both abilities.
func kboskBoth(t *testing.T, pol func(uint64) Policy) {
	t.Helper()
	var tal kboskTally
	kboskRun(t, pol, 1, 80, &tal, false)
	t.Logf("Murmuring Bosk on gorge's side: painful {W}/{B} %d, {G} %d", tal.pain, tal.green)
	if tal.pain == 0 || tal.green == 0 {
		t.Errorf("Murmuring Bosk activations: painful %d, {G} %d; want both", tal.pain, tal.green)
	}
}

// TestMurmuringBoskMutation: with the pair read as its first ability alone, a
// pain-only source (dirac.DebugPainFirstOff), some seed diverges, so
// TestMurmuringBoskLowers depends on the swapped pair and does not pass
// vacuously.
func TestMurmuringBoskMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPainFirstOff = true
	defer func() { dirac.DebugPainFirstOff = false }()
	if kboskRun(t, pkPolicy, 1, 40, nil, true) == nil && kboskRun(t, ExplorePolicy, 1, 48, nil, true) == nil {
		t.Error("no divergence with the pair read as its painful ability alone")
	}
}
