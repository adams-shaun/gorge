package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The attack tax's mana-source shapes in four-seat pods (CR 508.1d, 508.1g-i,
// 605; gorge rules/attack_cost.go attackManaSources, windowManaUnitsPaid,
// attackChoiceManaSources; kernel combat_attacktax.go). Every seat plays the
// one deck: a generic attack tax (Propaganda, Ghostly Prison) beside the
// shape, so each seat pays other seats' taxes through the payment window.
// The run tallies, at each window ask gorge poses, the sources the payer
// holds untapped ("present") and the one the oracle taps ("tap"); a shape's
// test requires its sources to have been reached, and the mutation to
// diverge.

// atkShapeStats is a run's tally.
type atkShapeStats struct {
	games, divs, stalls int
	tally               map[string]int
}

// runAtkShape plays seeds four-seat games of deck in every seat in
// lockstep, with mut set to on for the run.
func runAtkShape(t *testing.T, deck []cardset.Entry, seeds int, mut *bool, on bool) atkShapeStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var pool []*cards.Card
	for _, e := range deck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		pool = append(pool, c)
	}
	pd, err := kgen.TableFromCards(reg, pool, "atktax-shape-oracle")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, ids, err := set.MatchupN([][]cardset.Entry{deck, deck, deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	if dirac.TwoSeatOnly(tbl) {
		t.Fatalf("the table is still two-seat only: %v", dirac.TwoSeatCauses(tbl))
	}
	if r := kop.PairGuardN(tbl, 4); r != nil {
		t.Fatalf("the kernel refuses the pod: %s: %s", r.Reason, r.Why)
	}
	old := *mut
	*mut = on
	defer func() { *mut = old }()
	st := atkShapeStats{tally: map[string]int{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		m, err := KernelMatchupN(reg, tbl, []string{"pod-a", "pod-b", "pod-c", "pod-d"}, ids, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 40}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		var hookStats podClassStats
		pol := podClassPolicy(seed, func(c *PickCtx) bool {
			if gs == nil && c.G != nil {
				gs = c.G
				gs.OnInternal = func(d *decision.Decision, idx int) {
					atkShapeTally(gs.E.G, d, idx, st.tally)
				}
			}
			return false
		}, &hookStats)
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if !on {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			if !on {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return st
}

// atkShapeTally counts one attack-payment window ask: the window, the
// sources the payer holds untapped and the one the oracle taps.
func atkShapeTally(g *state.Game, d *decision.Decision, idx int, tally map[string]int) {
	if len(d.Options) == 0 || d.Options[0].Kind != "attack_mana" && d.Options[0].Kind != "block_mana" {
		return
	}
	tally["window taps"]++
	if d.Options[0].Kind == "block_mana" {
		tally["block taps"]++
	}
	for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(d.Player)) {
		if ob := g.Obj(id); ob != nil && ob.Face() != nil && !ob.Tapped {
			tally["present "+ob.Face().Name]++
		}
	}
	if o := optByIndex(d, idx); o.Obj != 0 {
		if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
			tally["tap "+ob.Face().Name]++
		}
	}
}

// atkShapeLockstep runs a shape's deck clean (no divergence, no stall, at
// least minTaps window taps, every name in present held untapped at a window
// ask and every name in tapped tapped by one) and under its mutation (some
// game diverges).
func atkShapeLockstep(t *testing.T, deck []cardset.Entry, seeds, minTaps, minBlock int, present, tapped []string, mut *bool) {
	t.Run("clean", func(t *testing.T) {
		st := runAtkShape(t, deck, seeds, mut, false)
		t.Logf("%d games, %d divergences, %d stalls; %s", st.games, st.divs, st.stalls, katTally(st.tally))
		if st.tally["window taps"] < minTaps {
			t.Errorf("only %d window taps, want >= %d", st.tally["window taps"], minTaps)
		}
		if st.tally["block taps"] < minBlock {
			t.Errorf("only %d block-window taps, want >= %d", st.tally["block taps"], minBlock)
		}
		for _, n := range present {
			if st.tally["present "+n] < 2 {
				t.Errorf("%s held untapped at %d window asks, want >= 2", n, st.tally["present "+n])
			}
		}
		for _, n := range tapped {
			if st.tally["tap "+n] < 2 {
				t.Errorf("%s tapped by %d window asks, want >= 2", n, st.tally["tap "+n])
			}
		}
	})
	t.Run("mutation", func(t *testing.T) {
		st := runAtkShape(t, deck, seeds, mut, true)
		if st.divs == 0 {
			t.Fatalf("mutation left %d games with no divergence", st.games)
		}
		t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
	})
}

// Sources the window never lists: a Signet's and a filter land's {1}, {T}
// (Azorius Signet, Sungrass Prairie), Prophetic Prism, a Vivid land's counter
// cost and Wall of Roots' counter. They add nothing to the budget and are
// never tapped (windowManaUnitsPaid lists a free {T} ability, a self-sacrifice
// or a fixed life cost only); the kernel's walk ignores them. The mutation
// (DebugAtkTaxCountUnlisted) counts them as fixed sources.
var atkTaxUnlistedDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Azorius Signet", Count: 2}, {Name: "Prophetic Prism", Count: 2}, {Name: "Wall of Roots", Count: 2},
	{Name: "Sungrass Prairie", Count: 2},
	{Name: "Plains", Count: 9}, {Name: "Island", Count: 7},
}

// atkTaxVividDeck swaps the Signet for a Vivid land, whose counter-cost
// ability is unlisted too (a Signet beside a Vivid land diverges in the cast
// planner with no attack tax in the deck, outside this class).
var atkTaxVividDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Prophetic Prism", Count: 2}, {Name: "Wall of Roots", Count: 2},
	{Name: "Sungrass Prairie", Count: 2}, {Name: "Vivid Creek", Count: 2},
	{Name: "Plains", Count: 9}, {Name: "Island", Count: 7},
}

// TestFourSeatAtkTaxUnlistedLockstep: four-seat attack-tax games with
// sources the attack window does not list play in lockstep, the payer
// holding them untapped as the window pays from its basic lands.
func TestFourSeatAtkTaxUnlistedLockstep(t *testing.T) {
	t.Run("signet", func(t *testing.T) {
		atkShapeLockstep(t, atkTaxUnlistedDeck, 24, 20, 0,
			[]string{"Azorius Signet", "Sungrass Prairie", "Prophetic Prism", "Wall of Roots"}, nil, &dirac.DebugAtkTaxCountUnlisted)
	})
	t.Run("vivid", func(t *testing.T) {
		atkShapeLockstep(t, atkTaxVividDeck, 24, 20, 0, []string{"Vivid Creek"}, nil, &dirac.DebugAtkTaxCountUnlisted)
	})
}

// Bounce lands ("{T}: Add {W}{U}", Produced$ "W U", ManaEach): gorge's window
// lists one fixed unit that makes both colours, so a tap pays two units and
// floats both colours (windowManaUnits: a non-choice production over its
// slots, CR 106.1, 605.3a). The mutation (DebugAtkTaxEachOne) reads one
// colour's unit.
var atkTaxBounceDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Azorius Chancery", Count: 4}, {Name: "Boros Garrison", Count: 2}, {Name: "Archangel of Tithes", Count: 2},
	{Name: "Plains", Count: 8}, {Name: "Island", Count: 4},
}

// TestFourSeatAtkTaxBounceLockstep: four-seat attack-tax games with bounce
// lands play in lockstep, the window tapping a bounce land for its two units.
func TestFourSeatAtkTaxBounceLockstep(t *testing.T) {
	atkShapeLockstep(t, atkTaxBounceDeck, 24, 20, 1, []string{"Azorius Chancery"}, []string{"Azorius Chancery"}, &dirac.DebugAtkTaxEachOne)
}

// Reflected mana ("{T}: Add one mana of any color that a land an opponent
// controls could produce": Fellwar Stone, Exotic Orchard). gorge lists the
// source in the window's fixed pass as one unit (a blank Produced$ is the
// executor's one-colourless default, windowManaUnits) but resolves its
// ManaReflected ability unpinned: with several colours on offer the colour ask
// it poses is displaced by the window's next tap ask, so the tap makes no
// mana, and when the window ran out of sources the abandoned declaration
// leaves the ask posed, answered with the first colour (rules
// attackPayAnswer, askManaColor). The mutation (DebugAtkTaxReflPin) pins the
// first colour at the tap.
var atkTaxReflDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Fellwar Stone", Count: 3}, {Name: "Exotic Orchard", Count: 2}, {Name: "Archangel of Tithes", Count: 2},
	{Name: "Plains", Count: 8}, {Name: "Island", Count: 4},
}

// TestFourSeatAtkTaxReflectedLockstep: four-seat attack-tax games with
// reflected-mana sources play in lockstep, the window tapping them.
func TestFourSeatAtkTaxReflectedLockstep(t *testing.T) {
	atkShapeLockstep(t, atkTaxReflDeck, 24, 20, 1, []string{"Fellwar Stone", "Exotic Orchard"},
		[]string{"Fellwar Stone", "Exotic Orchard"}, &dirac.DebugAtkTaxReflPin)
}

// Self-sacrifice sources: a Treasure ("{T}, Sacrifice this token: Add one mana
// of any color", Burdened Aerialist) and an Eldrazi Scion ("Sacrifice this
// creature: Add {C}", Eldrazi Skyspawner). gorge's paid-cost census lists them
// after every free fixed-production source and before the choice-shaped ones
// (Birds of Paradise; windowManaUnitsPaid, attackChoiceManaSources), the
// Treasure's colour pinned to its first alternative, {W}; a sacrifice alone
// needs no untapped source (CR 605.1a). The mutation (DebugAtkTaxPaidLast)
// taps them with the choice-shaped sources, in battlefield order.
var atkTaxPaidDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2},
	{Name: "Burdened Aerialist", Count: 4}, {Name: "Eldrazi Skyspawner", Count: 4}, {Name: "Birds of Paradise", Count: 3},
	{Name: "Savannah Lions", Count: 2}, {Name: "Elite Vanguard", Count: 2},
	{Name: "Plains", Count: 6}, {Name: "Island", Count: 5},
}

// TestFourSeatAtkTaxPaidLockstep: four-seat attack-tax games with Treasures
// and Eldrazi Spawn play in lockstep, the window sacrificing them.
func TestFourSeatAtkTaxPaidLockstep(t *testing.T) {
	atkShapeLockstep(t, atkTaxPaidDeck, 32, 20, 0, []string{"Treasure Token", "Eldrazi Scion Token"},
		[]string{"Treasure Token", "Eldrazi Scion Token"}, &dirac.DebugAtkTaxPaidLast)
}
