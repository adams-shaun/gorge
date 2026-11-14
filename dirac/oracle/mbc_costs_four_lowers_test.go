package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/decision"
)

// TestMBCCosts4Lowers: the Mystery Booster Commander costs-4 spelling family
// in lockstep, one subtest per spelling over the mbccosts4 fixture pool:
//
//   - alt-life: Snuff Out's "if you control a Swamp, you may pay 4 life rather
//     than pay this spell's mana cost" (AltCostIndex 1 cast, CR 118.9; kernel
//     cast_altfree.go altLifeCost);
//   - alt-life-board-raise: the same cast beside Kasmina's target-dependent tax,
//     its seat holding one land: the target menu prices each candidate with
//     the cast's alternative cost, not the printed {1}{B} (kernel
//     dropTargetsOwing, altBaseMana);
//   - alt-tap: Ramosian Rally's "tap an untapped creature you control rather
//     than pay this spell's mana cost" (kernel altTapCost);
//   - life-sac: Strands of Night's {B}{B}, Pay 2 life, Sacrifice a Swamp
//     (PayLife<N> beside Sac<N/Spec>, the life in Cost.Ctr: kgen
//     lifeBesideSac);
//   - discard-sac: Dreamscape Artist's {2}{U}, {T}, Discard a card, Sacrifice a
//     land (Discard<N> beside Sac<N> sharing Cost.N: kgen discardBesideSac;
//     gorge asks the sacrifice, then the discard).
//
// Each subtest's probe counts the gorge options its spelling exists for and
// fails when none was taken, so a refused card (the pool would not build) or a
// mechanism the games never reach fails as surely as a divergence.
func TestMBCCosts4Lowers(t *testing.T) {
	t.Parallel()
	altCast := func(card string) func(decision.Option, string) bool {
		return func(o decision.Option, name string) bool {
			return o.Kind == "cast" && o.AltCostIndex == 1 && o.Mode == "" && name == card
		}
	}
	activate := func(card string) func(decision.Option, string) bool {
		return func(o decision.Option, name string) bool { return o.Kind == "ability" && name == card }
	}
	for _, tc := range []struct {
		name  string
		decks [2][]cardset.Entry
		want  func(decision.Option, string) bool
		seeds uint64
		need  int
	}{
		{"alt-life", mbcSnuffOutDecks, altCast("Snuff Out"), 120, 20},
		{"alt-life-board-raise", mbcSnuffOutKasminaDecks, altCast("Snuff Out"), 300, 10},
		{"alt-tap", mbcRallyDecks, altCast("Ramosian Rally"), 120, 20},
		{"life-sac", mbcStrandsDecks, activate("Strands of Night"), 120, 10},
		{"discard-sac", mbcArtistDecks, activate("Dreamscape Artist"), 120, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			taken := 0
			pol := func(seed uint64) Policy {
				return mbcCosts3Probe(castFirstPolicy(seed), rand.New(rand.NewPCG(seed, 0x4c05)), tc.want, nil, &taken)
			}
			if div := lockstepRunPolicy(t, "mbccosts4", tc.decks, tc.seeds, true, nil, nil, pol); div != 0 {
				return // each divergence is already a test error
			}
			t.Logf("%s: %d gorge options of the spelling taken", tc.name, taken)
			if taken < tc.need {
				t.Fatalf("%s: %d options of the spelling taken over %d seeds, want at least %d: the pool does not exercise it", tc.name, taken, tc.seeds, tc.need)
			}
		})
	}
}

// mbcSnuffOutDecks: Swamps to enable the alternative cost, nonblack bodies to
// destroy.
var mbcSnuffOutDecks = [2][]cardset.Entry{
	{
		{Name: "Snuff Out", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Swamp", Count: 14}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Snuff Out", Count: 4}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Swamp", Count: 12}, {Name: "Forest", Count: 12},
	},
}

// mbcSnuffOutKasminaDecks: Kasmina taxes every spell its controller's
// opponents cast at a creature she or her controller keeps ({2} more). The
// Snuff Out seat has one Swamp and free Ornithopters: its two mana-less
// targets are its own, untaxed, and the printed {1}{B} is out of reach of one
// land, so only the cast's own alternative cost (4 life) leaves a target it
// can pay for. gorge prices each candidate with the cast's alternative cost
// (affordableTargetCandidates, CR 601.2c, 601.2f, 118.9); the kernel's menu
// must as well. No tax is ever paid (one land cannot), so the lands tapped
// stay out of the comparison.
var mbcSnuffOutKasminaDecks = [2][]cardset.Entry{
	{
		{Name: "Kasmina, Enigmatic Mentor", Count: 4}, {Name: "Ornithopter", Count: 12},
		{Name: "Island", Count: 24},
	},
	{
		{Name: "Snuff Out", Count: 12}, {Name: "Ornithopter", Count: 27},
		{Name: "Swamp", Count: 1},
	},
}

// mbcRallyDecks: Plains to enable the alternative cost, creatures to tap.
var mbcRallyDecks = [2][]cardset.Entry{
	{
		{Name: "Ramosian Rally", Count: 6}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Plains", Count: 14}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Ramosian Rally", Count: 4}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12},
	},
}

// mbcStrandsDecks: removal on both sides fills the graveyards Strands of
// Night returns a creature from.
var mbcStrandsDecks = [2][]cardset.Entry{
	{
		{Name: "Strands of Night", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Terror", Count: 4},
		{Name: "Swamp", Count: 16}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Strands of Night", Count: 2}, {Name: "Terror", Count: 4}, {Name: "Shock", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 8},
	},
}

// mbcArtistDecks: Dreamscape Artist beside cards to discard and basic lands
// to fetch and sacrifice.
var mbcArtistDecks = [2][]cardset.Entry{
	{
		{Name: "Dreamscape Artist", Count: 8}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Island", Count: 12}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Dreamscape Artist", Count: 4}, {Name: "Grizzly Bears", Count: 10}, {Name: "Shock", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 12},
	},
}
