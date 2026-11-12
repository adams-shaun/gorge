package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The charge-counter lands lockstep: Vivid Grove, Vivid Creek, Vivid Marsh,
// Vivid Crag, Vivid Meadow, Tendo Ice Bridge and Mirrodin's Core's second "{T},
// remove a charge counter: add one mana of any color" beside their fixed first ability is the card's counter gap
// (kernel mana_countercost.go), its source asking which ability and then the
// colour in gorge's prefix search. A W/R spell over a green, blue, black or
// colourless board is funded by the counter ability exactly as gorge's search
// funds it, and the counters run out after two (one on Tendo Ice Bridge).
// The first deck holds the gaps the prefix searches answer the colour ask
// over -- a restricted source (Giada's Angel-only {W}), an each-colour one
// (Bloom Tender), a restricted any-colour one (Secluded Courtyard) -- and
// Azorius First-Wing, whose {W}{U} the counter ability's second colour alone
// completes: the search answers the W branch first, which prices nothing, and
// then the U branch, which does.
var kvividDecks = [2][]cardset.Entry{
	{
		{Name: "Vivid Grove", Count: 3}, {Name: "Vivid Creek", Count: 3}, {Name: "Vivid Marsh", Count: 3},
		{Name: "Tendo Ice Bridge", Count: 2}, {Name: "Mirrodin's Core", Count: 2}, {Name: "Forest", Count: 2},
		{Name: "Plains", Count: 2}, {Name: "Secluded Courtyard", Count: 2},
		{Name: "Bloom Tender", Count: 2}, {Name: "Giada, Font of Hope", Count: 2}, {Name: "Azorius First-Wing", Count: 3},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Doom Blade", Count: 4}, {Name: "Serra Angel", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Wind Drake", Count: 2}, {Name: "Hill Giant", Count: 2},
	},
	{
		{Name: "Hill Giant", Count: 2}, {Name: "Wind Drake", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Doom Blade", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Serra Angel", Count: 2},
		{Name: "Azorius First-Wing", Count: 2},
		{Name: "Vivid Crag", Count: 3}, {Name: "Vivid Meadow", Count: 3}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

var kvividLands = map[string]bool{"Vivid Grove": true, "Vivid Creek": true, "Vivid Marsh": true, "Tendo Ice Bridge": true, "Mirrodin's Core": true,
	"Vivid Crag": true, "Vivid Meadow": true}

// kvividRun plays seeds with pol, failing on each divergence or unmodelled
// stall (unless mutated: the first divergence is returned instead). byName counts, from gorge's side, each land's counter-cost mana
// activations (the any-colour second ability).
func kvividRun(t *testing.T, pol func(uint64) Policy, n uint64, byName map[string]int, mutated bool) *Divergence {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("vividlands")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kvividDecks {
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
	tbl, _, _, err := set.Matchup(kvividDecks[0], kvividDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"vl-a", "vl-b"}, decks, 1)
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
					if sa.API != "Mana" || !strings.Contains(sa.ParamStr(cards.PKCost), "SubCounter") {
						return
					}
					if o := e.G.Obj(src); o != nil && o.Face() != nil && kvividLands[o.Face().Name] {
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
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return nil
}

// TestVividCounterLandsLockstep: random and explore policies, no divergence and
// no unmodelled stall, and the explore run removes a charge counter for mana on
// gorge's side so the lockstep actually compares the counter ability's walk.
func TestVividCounterLandsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	kvividRun(t, pkPolicy, 48, nil, false)
	byName := map[string]int{}
	kvividRun(t, ExplorePolicy, 64, byName, false)
	for name := range kvividLands {
		t.Logf("%s counter ability activated %d times on gorge's side", name, byName[name])
		if byName[name] == 0 {
			t.Errorf("%s's charge-counter mana ability was never activated on gorge's side", name)
		}
	}
}

// TestVividCounterLandsMutation: with the counter-cost gaps hidden
// (dirac.DebugNoCtrMana) the explore run diverges, so TestVividCounterLandsLockstep
// depends on the kernel playing the counter ability, and does not pass vacuously.
func TestVividCounterLandsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCtrMana = true
	defer func() { dirac.DebugNoCtrMana = false }()
	if kvividRun(t, ExplorePolicy, 64, nil, true) == nil {
		t.Error("no divergence without the counter-cost gap")
	}
}
