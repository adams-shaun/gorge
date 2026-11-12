package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The chosen-colour second mana ability lockstep: Quirion Elves' second
// "{T}: Add one mana of the chosen colour" beside its fixed {G} ability is
// one source unit whose production is the printed colour beside the chosen
// one, in the fixed-production census pass (pcard.manaFixedChosen). Its
// chosen colour is the ETB ChooseColor answer, so a W/R/U/B spell over a
// green board is paid by the second ability exactly as gorge's planner pays
// it. The kernel's payment must tap the same Quirion when two are on the
// board (measured: the choice-shaped pass tapped the second, gorge the first).
var kmanaonlyDecks = [2][]cardset.Entry{
	{
		{Name: "Quirion Elves", Count: 2}, {Name: "Nomadic Elf", Count: 2}, {Name: "Orochi Leafcaller", Count: 2},
		{Name: "Agent of Stromgald", Count: 1}, {Name: "Bog Initiate", Count: 1}, {Name: "Energy Refractor", Count: 2},
		{Name: "Prismite", Count: 1}, {Name: "Crossroads Candleguide", Count: 1}, {Name: "Signpost Scarecrow", Count: 1},
		{Name: "Urn of Godfire", Count: 1}, {Name: "Llanowar Envoy", Count: 1},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Doom Blade", Count: 1}, {Name: "Serra Angel", Count: 1},
		{Name: "Lightning Bolt", Count: 1}, {Name: "Wind Drake", Count: 2}, {Name: "Hill Giant", Count: 1},
		{Name: "Forest", Count: 4}, {Name: "Swamp", Count: 3}, {Name: "Plains", Count: 3},
		{Name: "Mountain", Count: 4}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Doom Blade", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Serra Angel", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

// kmanaonlyFilters are the pool's mana-only-fee carriers and Quirion Elves.
var kmanaonlyFilters = map[string]bool{"Quirion Elves": true, "Nomadic Elf": true, "Orochi Leafcaller": true,
	"Agent of Stromgald": true, "Bog Initiate": true, "Energy Refractor": true, "Prismite": true,
	"Crossroads Candleguide": true, "Signpost Scarecrow": true, "Urn of Godfire": true, "Llanowar Envoy": true}

// kmanaonlyRun plays seeds with pol, failing on each divergence or unmodelled
// stall, and returns the number of games it played. byName counts, from
// gorge's side, each carrier's mana-ability activations.
func kmanaonlyRun(t *testing.T, pol func(uint64) Policy, n uint64, byName map[string]int) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("manaonly")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kmanaonlyDecks {
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
	tbl, _, _, err := set.Matchup(kmanaonlyDecks[0], kmanaonlyDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mo-a", "mo-b"}, decks, 1)
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
					if sa.API != "Mana" {
						return
					}
					if o := e.G.Obj(src); o != nil && o.Face() != nil && kmanaonlyFilters[o.Face().Name] {
						byName[o.Face().Name]++
					}
				}
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 20, Check: check})
		CloseSide(k)
		seen++
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
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
	return seen
}

// TestManaOnlyFilterLockstep: random and explore policies, no divergence and
// no unmodelled stall, and the explore run activates every carrier at least
// once so the lockstep actually compares its mana walk.
func TestManaOnlyFilterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	if n := kmanaonlyRun(t, pkPolicy, 48, nil); n != 48 {
		t.Fatalf("played %d of 48 seeds", n)
	}
	byName := map[string]int{}
	kmanaonlyRun(t, ExplorePolicy, 64, byName)
	for name := range kmanaonlyFilters {
		t.Logf("%s activated %d times on gorge's side", name, byName[name])
		if byName[name] == 0 {
			t.Errorf("%s was never activated on gorge's side", name)
		}
	}
}
