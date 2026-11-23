package oracle

import (
	"slices"
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

// kmanaonlyRun plays seeds from through to on dk with pol, failing on each
// divergence or unmodelled stall, and returns the number of games it played.
// byName counts, from gorge's side, each carrier's mana-ability activations.
func kmanaonlyRun(t *testing.T, dk [2][]cardset.Entry, pol func(uint64) Policy, from, to uint64, byName map[string]int) int {
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
	for s, d := range dk {
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
	tbl, _, _, err := set.Matchup(dk[0], dk[1])
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for seed := from; seed <= to; seed++ {
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

// TestManaOnlyFilterLockstep: the random policy over 48 seeds, no divergence
// and no unmodelled stall.
func TestManaOnlyFilterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	if n := kmanaonlyRun(t, kmanaonlyDecks, pkPolicy, 1, 48, nil); n != 48 {
		t.Fatalf("played %d of 48 seeds", n)
	}
}

// TestManaOnlyFilterExploreLockstep: the explore policy over 64 seeds, no
// divergence and no unmodelled stall (split from TestManaOnlyFilterLockstep
// to fit the test budget). Which carriers the run reaches moves with every
// decision the bots' choices shift, so it only logs their activation counts;
// TestManaOnlyCarrierForcedLines holds each carrier to an activation.
func TestManaOnlyFilterExploreLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	byName := map[string]int{}
	kmanaonlyRun(t, kmanaonlyDecks, ExplorePolicy, 1, 64, byName)
	for _, name := range kmanaonlyCarriers() {
		t.Logf("%s activated %d times on gorge's side", name, byName[name])
	}
}

// kmanaonlyCarriers is kmanaonlyFilters' names in order.
func kmanaonlyCarriers() []string {
	names := make([]string, 0, len(kmanaonlyFilters))
	for name := range kmanaonlyFilters {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// kmanaonlyForced is the first deck with every other carrier replaced by
// copies of carrier and every basic land by the one basic that feeds it
// (Mountain for Agent of Stromgald's {R}, else Forest): the off-colour spells
// stay, so paying for them wants the carrier's filtered mana. Agent of
// Stromgald, which makes only {B}, gets Doom Blades for its spells, the
// pool's one black spell.
func kmanaonlyForced(carrier string) [2][]cardset.Entry {
	land := "Forest"
	black := carrier == "Agent of Stromgald"
	if black {
		land = "Mountain"
	}
	var d []cardset.Entry
	extra, lands, spells := 0, 0, 0
	for _, e := range kmanaonlyDecks[0] {
		switch {
		case kmanaonlyBasic[e.Name]:
			lands += e.Count
		case kmanaonlyFilters[e.Name] && e.Name != carrier:
			extra += e.Count
		case black && !kmanaonlyFilters[e.Name]:
			spells += e.Count
		default:
			d = append(d, e)
		}
	}
	if spells > 0 {
		d = append(d, cardset.Entry{Name: "Doom Blade", Count: spells})
	}
	for i := range d {
		if d[i].Name == carrier {
			d[i].Count += extra
		}
	}
	d = append(d, cardset.Entry{Name: land, Count: lands})
	return [2][]cardset.Entry{d, kmanaonlyDecks[1]}
}

var kmanaonlyBasic = map[string]bool{"Forest": true, "Swamp": true, "Plains": true, "Mountain": true, "Island": true}

// kmanaonlyForcedExempt are the carriers no forced line reaches. Bog
// Initiate's "{1}: Add {B}" is a fee-only source: gorge's autopay takes it
// only when no cheaper source pays the {B}, and the canonical surface offers
// no manual mana activation, nor can a lockstep game stack a library, to
// force it (none of 12 seeds on a one-Swamp deck, nor 6 on an Agent of
// Stromgald deck, activated it). engine/rules' TestRepeatableFeeOnlyManaFundsTwoPips
// and TestRepeatableFeeOnlyBesideRefractorFundsTwoPips cover the line gorge
// offers through it.
var kmanaonlyForcedExempt = map[string]bool{"Bog Initiate": true}

// TestManaOnlyCarrierForcedLines is each carrier's forced line: the explore
// bot on a deck stacked with that carrier (kmanaonlyForced), seed by seed
// until gorge activates the carrier's mana ability, so the lockstep compares
// every carrier's mana walk whatever the 64-seed sweep reaches. Each game is
// a full lockstep, failing on a divergence or an unmodelled stall.
func TestManaOnlyCarrierForcedLines(t *testing.T) {
	if isolate(t) {
		return
	}
	const tries = 4
	for _, name := range kmanaonlyCarriers() {
		if kmanaonlyForcedExempt[name] {
			continue
		}
		var got int
		for seed := uint64(1); seed <= tries && got == 0; seed++ {
			byName := map[string]int{}
			kmanaonlyRun(t, kmanaonlyForced(name), ExplorePolicy, seed, seed, byName)
			got = byName[name]
		}
		if got == 0 {
			t.Errorf("%s was never activated on gorge's side in %d forced games", name, tries)
		}
	}
}
