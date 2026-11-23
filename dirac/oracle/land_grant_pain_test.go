package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// A painland under Stormtide Leviathan (CR 305.6, 605.3b): gorge lists its
// printed {C}, the intrinsic {U}, then the painful colour choice (last resort,
// pay.PaymentPlanDamageRider). The normal alternatives merge into one free unit
// of {C}|{U} with the intrinsic first, the painful one stays the planner's pair.
var landGrantPainLands = []string{
	"Battlefield Forge", "Caves of Koilos", "Sulfurous Springs", "Underground River", "Karplusan Forest", "Shivan Reef", "Yavimaya Coast",
}

func landGrantPainDeck() []string {
	return append(slices.Clone(landGrantAltShared), landGrantPainLands...)
}

func landGrantPainSrc(name string, _ int) bool { return slices.Contains(landGrantPainLands, name) }

// TestFourSeatLandGrantPainLockstep: no seed diverges or stalls, and the
// Leviathan meets untapped painlands.
func TestFourSeatLandGrantPainLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantRun(t, landGrantPainDeck(), 120, landGrantPainSrc)
	if reached == 0 {
		t.Fatal("the Leviathan never met an untapped painland")
	}
	t.Logf("%d decision points with the Leviathan beside an untapped painland", reached)
}

// TestLandGrantPainOffRefused: with the pain view off the refusal names the
// painlands and the Leviathan.
func TestLandGrantPainOffRefused(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantPainOff = true
	defer func() { dirac.DebugLandGrantPainOff = false }()
	landGrantRefusedNames(t, landGrantPainDeck(), []string{"Stormtide Leviathan", "Battlefield Forge", "Shivan Reef"}, []string{"Golgari Rot Farm", "Temple of the False God"})
}

// TestLandGrantPainViewMutation: with the painland's view keeping its printed {C}
// alone, gorge's plans pay a {U} through the intrinsic ability and the pod diverges.
func TestLandGrantPainViewMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantPainView = true
	defer func() { dirac.DebugLandGrantPainView = false }()
	set := edhSet(t)
	deck := landGrantPainDeck()
	pod := [4][]string{deck, deck, deck, deck}
	play := fourSeatSetPlay(t, set, pod, nil, nil, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 60; seed++ {
		if res := play(seed); res.Div != nil || res.StallUnmodelled {
			return
		}
	}
	t.Fatal("no seed diverged with the painland's view dropping the intrinsic colour")
}
