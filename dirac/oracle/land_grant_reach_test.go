package oracle

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// A granted basic land type only reaches the cards its granter's filter names
// (CR 305.6, 613.1d): Ashaya, Soul of the Wild makes nontoken creatures Forest
// lands, so Sol Ring, a Signet or a Talisman beside it gains no intrinsic
// "{T}: Add {G}" and the pair is no refusal. The refusal was an
// over-approximation of the filter (static_landgrantpair.go landGrantScan),
// keeping every Ashaya pod holding any non-plain mana source out of a deal.
//
// Citanul Hierophants gives every creature "{T}: Add {G}" and stays refused
// beside Ashaya (TestLandGrantGrantedManaRefused). Arixmethes ("{T}: Add {G}{U}")
// does too: gorge appends the Forest's {G} unless an ability has Produced$
// exactly G (ManaAbilitiesProduce keys on the raw text), so an all-at-once
// ability does not make it.
var landGrantReachShared = []string{
	"Sol Ring", "Arcane Signet", "Orzhov Signet", "Simic Signet", "Talisman of Progress", "Talisman of Conviction",
	"Everflowing Chalice", "Hedron Archive", "Thought Vessel", "Mind Stone",
	"Forest", "Forest", "Forest", "Forest", "Forest", "Island", "Island", "Island", "Plains", "Plains", "Swamp", "Swamp",
	"Temple of the False God", "Azorius Chancery", "Simic Growth Chamber", "Battlefield Forge",
}

var landGrantReachAshaya = append(slices.Clone(landGrantReachShared),
	"Ashaya, Soul of the Wild", "Augur of Autumn", "Body of Knowledge",
	"Tatyova, Benthic Druid", "Llanowar Elves", "Elvish Archdruid", "Priest of Titania")

var landGrantReachPlain = append(slices.Clone(landGrantReachShared),
	"Heronblade Elite", "Aesi, Tyrant of Gyre Strait", "Llanowar Elves", "Elvish Mystic", "Birds of Paradise", "Wall of Omens", "Coiling Oracle",
	"Kodama's Reach", "Cultivate", "Rampant Growth", "Counterspell", "Beast Within")

// landGrantReachRun plays seeds of the Ashaya pod and counts the games in
// which Ashaya, and a non-plain mana source beside it, reached the battlefield.
func landGrantReachRun(t *testing.T, seeds uint64) (reached, bad int) {
	t.Helper()
	set := edhSet(t)
	check := func(g *GorgeSide, _ Side) *Divergence {
		var ash, src bool
		for p := range 4 {
			for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
					switch o.Face().Name {
					case "Ashaya, Soul of the Wild":
						ash = true
					case "Sol Ring", "Arcane Signet", "Orzhov Signet", "Simic Signet", "Talisman of Progress":
						src = true
					}
				}
			}
		}
		if ash && src {
			reached++
		}
		return nil
	}
	play := fourSeatSetPlay(t, set, [4][]string{landGrantReachAshaya, landGrantReachPlain, landGrantReachAshaya, landGrantReachPlain},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil {
			bad++
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
		if res.StallUnmodelled || strings.Contains(res.Stall, "landgrant") {
			bad++
			t.Errorf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	return reached, bad
}

// TestFourSeatLandGrantAshayaReachLockstep plays the Ashaya pod: it builds (no
// landgrant refusal), no seed diverges or stalls, and Ashaya meets a Signet
// or Sol Ring on the battlefield.
func TestFourSeatLandGrantAshayaReachLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantReachRun(t, 24)
	if reached == 0 {
		t.Fatal("Ashaya never met a non-plain mana source: the pair is unexercised")
	}
	t.Logf("%d decision points with Ashaya beside a non-plain mana source", reached)
}

// TestLandGrantReachMutation: with the grant reaching every card again
// (DebugLandGrantAnyReach, the old over-approximation) the same pod is refused
// at its start, so the filter is what lets it play.
func TestLandGrantReachMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugLandGrantAnyReach
	dirac.DebugLandGrantAnyReach = true
	defer func() { dirac.DebugLandGrantAnyReach = old }()
	set := edhSet(t)
	var lists [][]cardset.Entry
	for _, d := range [][]string{landGrantReachAshaya, landGrantReachPlain, landGrantReachAshaya, landGrantReachPlain} {
		lists = append(lists, landGrantEntries(d))
	}
	_, _, err := set.MatchupN(lists)
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != "landgrant" {
		t.Fatalf("with the over-approximation on the pod builds or fails otherwise: %v, want the landgrant refusal", err)
	}
}

// TestLandGrantGrantedManaRefused: Ashaya beside Citanul Hierophants is still
// refused, naming both: a creature with the granted "{T}: Add {G}" and the
// Forest's intrinsic one is one census unit in gorge whose position the
// kernel's classless granted unit does not follow, and the pod diverged on the
// payment of Ashaya's own cast (seed 1 of the pod below, before the refusal).
func TestLandGrantGrantedManaRefused(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantOwnOff = true // the pair is modelled now (blgOwnUsable); the switch restores the refusal
	defer func() { dirac.DebugLandGrantOwnOff = false }()
	set := edhSet(t)
	hier := slices.Clone(landGrantReachAshaya)
	hier = append(hier, "Citanul Hierophants")
	var lists [][]cardset.Entry
	for _, d := range [][]string{hier, landGrantReachPlain, hier, landGrantReachPlain} {
		lists = append(lists, landGrantEntries(d))
	}
	_, _, err := set.MatchupN(lists)
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != "landgrant" {
		t.Fatalf("Ashaya beside Citanul Hierophants: %v, want the landgrant refusal", err)
	}
	for _, n := range []string{"Ashaya, Soul of the Wild", "Citanul Hierophants"} {
		if !slices.Contains(ir.Cards, n) {
			t.Errorf("refusal names %v, want %s among them", ir.Cards, n)
		}
	}
	if slices.Contains(ir.Cards, "Sol Ring") {
		t.Errorf("refusal names Sol Ring (%v): a non-creature is out of Ashaya's reach", ir.Cards)
	}
}
