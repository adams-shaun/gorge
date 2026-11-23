package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// A gated land under Stormtide Leviathan (CR 305.6, 605.3b): Tainted Field's
// "{T}: Add {W} or {B}. Activate only if you control a Swamp" and Spire of
// Industry's "{T}, Pay 1 life: Add one mana of any color. Activate only if you
// control an artifact" beside their {C}, and the intrinsic {U}. Sol Ring and
// Arcane Signet (artifacts) and the Swamps are in the shared list.
var landGrantGateLands = []string{"Tainted Field", "Tainted Peak", "Spire of Industry"}

func landGrantGateDeck() []string {
	return append(slices.Clone(landGrantAltShared), landGrantGateLands...)
}

func landGrantGateSrc(name string, _ int) bool { return slices.Contains(landGrantGateLands, name) }

// TestFourSeatLandGrantGateLockstep: no seed diverges or stalls, and the
// Leviathan meets untapped gated lands.
func TestFourSeatLandGrantGateLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantRun(t, landGrantGateDeck(), 120, landGrantGateSrc)
	if reached == 0 {
		t.Fatal("the Leviathan never met an untapped gated land")
	}
	t.Logf("%d decision points with the Leviathan beside an untapped gated land", reached)
}

// TestLandGrantGateOffRefused: with the merged views off the refusal names the
// gated lands and the Leviathan.
func TestLandGrantGateOffRefused(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantGateOff = true
	defer func() { dirac.DebugLandGrantGateOff = false }()
	landGrantRefusedNames(t, landGrantGateDeck(), []string{"Stormtide Leviathan", "Tainted Field", "Spire of Industry"}, []string{"Golgari Rot Farm", "Temple of the False God"})
}

// TestLandGrantGateViewMutation: with the gate views keeping the printed colours
// alone the pod diverges (gorge pays a {U} through the intrinsic ability).
func TestLandGrantGateViewMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantGateView = true
	defer func() { dirac.DebugLandGrantGateView = false }()
	set := edhSet(t)
	deck := landGrantGateDeck()
	pod := [4][]string{deck, deck, deck, deck}
	play := fourSeatSetPlay(t, set, pod, nil, nil, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 60; seed++ {
		if res := play(seed); res.Div != nil || res.StallUnmodelled {
			return
		}
	}
	t.Fatal("no seed diverged with the gate views dropping the intrinsic colour")
}
