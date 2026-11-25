package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// graveReduceProbe is Convergence of Dominion's graveyard activation
// reduction without its commander gate (IsPresent$ Card.IsCommander never
// holds in a game with no commander, so the corpus card prices nothing here):
// "activated abilities of cards in your graveyard cost {2} less, to a minimum
// of one mana". Hellspark Elemental's unearth {1}{R} (CR 702.84a) is the
// graveyard activation it reduces to {R}.
const graveReduceProbe = `Name:Grave Reduce Probe
ManaCost:1
Types:Artifact
S:Mode$ ReduceCost | ValidCard$ Card.YouOwn | Type$ Ability | Amount$ 2 | MinMana$ 1 | AffectedZone$ Graveyard | Description$ Activated abilities of cards in your graveyard cost {2} less to activate. This effect can't reduce the mana in that ability's activation cost to less than one mana.
Oracle:probe
`

var graveReduceDecks = [2][]cardset.Entry{
	{
		{Name: "Grave Reduce Probe", Count: 4}, {Name: "Hellspark Elemental", Count: 8},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Raging Goblin", Count: 8}, {Name: "Mountain", Count: 16},
	},
	{
		{Name: "Grave Reduce Probe", Count: 3}, {Name: "Hellspark Elemental", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Raging Goblin", Count: 11}, {Name: "Mountain", Count: 16},
	},
}

// graveReduceEmbalmDecks pair the probe with Cursecloth Wrappings' granted
// embalm (CR 702.128a; kw_embalmgrant.go), which the graveyard static prices
// like any graveyard activation: the offer and the charge must agree. Deck A
// casts Cursecloth off Swamps and its {1}{B} creatures have generic mana
// above the MinMana$ 1 floor, so the reduction changes their embalm price.
var graveReduceEmbalmDecks = [2][]cardset.Entry{
	{
		{Name: "Grave Reduce Probe", Count: 4}, {Name: "Cursecloth Wrappings", Count: 6}, {Name: "Walking Corpse", Count: 6},
		{Name: "Bog Imp", Count: 4}, {Name: "Scathe Zombies", Count: 4}, {Name: "Swamp", Count: 16},
	},
	{
		{Name: "Lightning Bolt", Count: 8}, {Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Forest", Count: 8},
	},
}

// graveReduceLockstep plays the probe's decks in lockstep to the end of seeds
// 1..n and returns the divergent seeds.
func graveReduceLockstep(t *testing.T, n uint64, stopAtFirst bool) []uint64 {
	t.Helper()
	return graveReduceLockstepDecks(t, graveReduceDecks, n, stopAtFirst)
}

func graveReduceLockstepDecks(t *testing.T, graveReduceDecks [2][]cardset.Entry, n uint64, stopAtFirst bool) []uint64 {
	t.Helper()
	corpus := testutil.CorpusRegistry(t)
	reg := cards.NewRegistry()
	for k, v := range corpus.Tokens {
		reg.Tokens[k] = v
	}
	probe, diags := cards.ParseBytes("probe.txt", []byte(graveReduceProbe))
	if len(diags) != 0 {
		t.Fatalf("probe: %v", diags)
	}
	probe.Link()
	for _, fc := range probe.Faces {
		fc.ApplyIntrinsics()
	}
	reg.Add(probe)
	var decks [2][]*cards.Card
	var pool []*cards.Card
	seen := map[string]bool{}
	for s, d := range graveReduceDecks {
		for _, e := range d {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				if cd, ok = corpus.Lookup(e.Name); !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				reg.Add(cd)
			}
			if !seen[e.Name] {
				seen[e.Name] = true
				pool = append(pool, cd)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], cd)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	pd, err := kgen.TableFromCards(reg, pool, "gravereduce-probe")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	tbl, _, _, err := set.Matchup(graveReduceDecks[0], graveReduceDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var div []uint64
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"gravereduce-a", "gravereduce-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 24})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div = append(div, seed)
			if stopAtFirst {
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

// TestStaticZonesGraveReduceLockstep: an unearth from the graveyard is priced
// {2} less under a graveyard ReduceCost static, at the offer and the charge
// (cost_reduceability.go abReducedGrave); AbReducedPriced counts the charges
// the static changed.
func TestStaticZonesGraveReduceLockstep(t *testing.T) {
	// isolate: AbReducedPriced is a process global other lockstep tests add to.
	if isolate(t) {
		return
	}
	before := dirac.AbReducedPriced.Load()
	graveReduceLockstep(t, 60, false)
	if n := dirac.AbReducedPriced.Load() - before; n < 5 {
		t.Fatalf("only %d graveyard activations priced under the static: the pool does not exercise it", n)
	}
}

// TestStaticZonesGraveReduceMutation: with the reduction dropped some seed
// diverges, so the pool does activate a reduced graveyard ability.
func TestStaticZonesGraveReduceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoAbReduce = true
	defer func() { dirac.DebugNoAbReduce = false }()
	if len(graveReduceLockstep(t, 60, true)) == 0 {
		t.Fatal("no seed diverged with the ability reduction dropped: the pool never activates a reduced graveyard ability")
	}
}

// TestStaticZonesGraveReduceEmbalmLockstep: a granted embalm is offered at the
// price the graveyard static leaves, the price the charge takes (finding: the
// embalm offer once priced the card's full mana cost).
func TestStaticZonesGraveReduceEmbalmLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	before := dirac.AbReducedPriced.Load()
	graveReduceLockstepDecks(t, graveReduceEmbalmDecks, 60, false)
	if n := dirac.AbReducedPriced.Load() - before; n < 3 {
		t.Fatalf("only %d graveyard activations priced under the static: the pool does not exercise a granted embalm", n)
	}
}
