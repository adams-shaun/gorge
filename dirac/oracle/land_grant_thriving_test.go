package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// A Thriving land under Stormtide Leviathan (CR 305.6, 605.3b): "{T}: Add {W}
// or one mana of the chosen color" beside the granted Island's intrinsic {U}.
// The raw Produced$ ("Combo W Chosen") is no single colour, so gorge lists the
// intrinsic ability beside it whatever the grant, and the source ranks over the
// union of the fixed, the chosen and the granted colour.
var landGrantThrivingLands = []string{"Thriving Heath", "Thriving Bluff", "Thriving Grove", "Thriving Moor"}

func landGrantThrivingDeck() []string {
	d := slices.Clone(landGrantAltShared)
	for _, n := range landGrantThrivingLands {
		d = append(d, n, n)
	}
	return d
}

func landGrantThrivingSrc(name string, _ int) bool {
	return slices.Contains(landGrantThrivingLands, name)
}

// TestFourSeatLandGrantThrivingLockstep: no seed diverges or stalls, and the
// Leviathan meets untapped Thriving lands.
func TestFourSeatLandGrantThrivingLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantRun(t, landGrantThrivingDeck(), 120, landGrantThrivingSrc)
	if reached == 0 {
		t.Fatal("the Leviathan never met an untapped Thriving land")
	}
	t.Logf("%d decision points with the Leviathan beside an untapped Thriving land", reached)
}

// TestLandGrantThrivingOffRefused: with the Thriving view off the refusal names
// the Thriving lands and the Leviathan, and not the pair, pain or gated lands.
func TestLandGrantThrivingOffRefused(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantThrivingOff = true
	defer func() { dirac.DebugLandGrantThrivingOff = false }()
	landGrantRefusedNames(t, landGrantThrivingDeck(), []string{"Stormtide Leviathan", "Thriving Heath", "Thriving Moor"}, []string{"Golgari Rot Farm", "Temple of the False God"})
}

// TestLandGrantThrivingViewMutation: with the view keeping the printed colours
// alone the pod diverges (gorge pays a {U} through the intrinsic ability).
func TestLandGrantThrivingViewMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantThrivingView = true
	defer func() { dirac.DebugLandGrantThrivingView = false }()
	set := edhSet(t)
	deck := landGrantThrivingDeck()
	pod := [4][]string{deck, deck, deck, deck}
	play := fourSeatSetPlay(t, set, pod, nil, nil, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 60; seed++ {
		if res := play(seed); res.Div != nil || res.StallUnmodelled {
			return
		}
	}
	t.Fatal("no seed diverged with the Thriving view dropping the intrinsic colour")
}
