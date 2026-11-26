package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The restricted colourless pair lockstep: Eldrazi Temple's "{T}: Add {C}{C}",
// spendable only on colorless Eldrazi spells and abilities of colorless
// Eldrazi, beside its plain "{T}: Add {C}". gorge's planner prices both
// alternatives of the one unit; the kernel's restricted pair unit widens the
// census {C} to the larger colourless one (mana_restrictedsource.go
// prepRestrSrc), admitted per payment class by the restriction's spell half
// (Matter Reshaper, Thought-Knot Seer) or activation half (Endbringer's and
// Kozilek's Pathfinder's {C} abilities), never for Memnite, Hangarback Walker
// or the coloured spells.
var ktempleDecks = [2][]cardset.Entry{
	{
		{Name: "Eldrazi Temple", Count: 8}, {Name: "Wastes", Count: 6}, {Name: "Forest", Count: 3},
		{Name: "Eldrazi Mimic", Count: 4}, {Name: "Matter Reshaper", Count: 4}, {Name: "Thought-Knot Seer", Count: 3},
		{Name: "Endbringer", Count: 2}, {Name: "Kozilek's Pathfinder", Count: 2}, {Name: "Memnite", Count: 2},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hangarback Walker", Count: 3},
	},
	{
		{Name: "Eldrazi Temple", Count: 6}, {Name: "Wastes", Count: 6}, {Name: "Mountain", Count: 5},
		{Name: "Eldrazi Mimic", Count: 3}, {Name: "Reality Smasher", Count: 2}, {Name: "Kozilek's Pathfinder", Count: 2},
		{Name: "Endbringer", Count: 2}, {Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Memnite", Count: 3}, {Name: "Ulamog's Crusher", Count: 2}, {Name: "Matter Reshaper", Count: 2},
	},
}

// ktempleRun plays seeds with pol, failing on each divergence or unmodelled
// stall (unless mutated: the first divergence is returned instead). n2 counts,
// from gorge's side, Eldrazi Temple's restricted {C}{C} activations.
func ktempleRun(t *testing.T, pol func(uint64) Policy, n uint64, n2 *int, mutated bool) *Divergence {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("eldrazitemple")
	if err != nil {
		t.Fatal(err)
	}
	kd := ktempleDecks
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
		m, err := NewMatchup(reg, [2]string{"et-a", "et-b"}, decks, 1)
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
			if ref.E.ManaAbilityHook == nil && n2 != nil {
				e := ref.E
				e.ManaAbilityHook = func(_ state.PlayerID, src state.ObjID, sa *cards.SA) {
					if sa.API != "Mana" || !sa.HasParam(cards.PKRestrictValid) {
						return
					}
					if o := e.G.Obj(src); o != nil && o.Face() != nil && o.Face().Name == "Eldrazi Temple" {
						*n2++
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

// TestEldraziTempleLowers: random and explore policies, no divergence and no
// unmodelled stall, and the explore run taps Eldrazi Temple for its restricted
// {C}{C} on gorge's side, so the lockstep compares the pair's larger
// alternative.
func TestEldraziTempleLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	ktempleRun(t, pkPolicy, 48, nil, false)
	n2 := 0
	ktempleRun(t, ExplorePolicy, 64, &n2, false)
	t.Logf("Eldrazi Temple's restricted {C}{C} activated %d times on gorge's side", n2)
	if n2 == 0 {
		t.Error("Eldrazi Temple's restricted ability was never activated on gorge's side")
	}
}

// TestEldraziTempleMutation: with every restricted source admitted for every
// payment (dirac.DebugRestrictedSource = 1) the explore run diverges, so
// TestEldraziTempleLowers depends on the kernel holding the {C}{C} to colorless
// Eldrazi and does not pass vacuously.
func TestEldraziTempleMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRestrictedSource = 1
	defer func() { dirac.DebugRestrictedSource = 0 }()
	if ktempleRun(t, ExplorePolicy, 64, nil, true) == nil {
		t.Error("no divergence with the restriction ignored")
	}
}
