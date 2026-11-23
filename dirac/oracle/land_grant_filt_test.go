package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// A filter land under Stormtide Leviathan (CR 305.6, 605.3b): "{1},{T}: Add {G}{W}"
// is a paid ability gorge's planner prices as a census gap, and the intrinsic
// {U} beside it is the land's plain census unit; the prefix search answers the
// printed filter before the intrinsic tap.
var landGrantFiltLands = []string{"Sungrass Prairie", "Mossfire Valley", "Desolate Mire", "Shadowblood Ridge", "Sunscorched Divide", "Skycloud Expanse"}

func landGrantFiltDeck() []string {
	return append(slices.Clone(landGrantAltShared), landGrantFiltLands...)
}

func landGrantFiltSrc(name string, _ int) bool { return slices.Contains(landGrantFiltLands, name) }

// TestFourSeatLandGrantFiltLockstep: no seed diverges or stalls, and the
// Leviathan meets untapped filter lands.
func TestFourSeatLandGrantFiltLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantRun(t, landGrantFiltDeck(), 120, landGrantFiltSrc)
	if reached == 0 {
		t.Fatal("the Leviathan never met an untapped filter land")
	}
	t.Logf("%d decision points with the Leviathan beside an untapped filter land", reached)
}

// TestLandGrantFiltOffRefused: with the filter view off the refusal names the
// filter lands and the Leviathan.
func TestLandGrantFiltOffRefused(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantFiltOff = true
	defer func() { dirac.DebugLandGrantFiltOff = false }()
	landGrantRefusedNames(t, landGrantFiltDeck(), []string{"Stormtide Leviathan", "Sungrass Prairie", "Sunscorched Divide"}, []string{"Golgari Rot Farm", "Temple of the False God"})
}

// landGrantFiltMutated plays the filter pod under a mutation and reports whether
// some seed diverged or stalled.
func landGrantFiltMutated(t *testing.T, deck []string, seeds uint64) bool {
	t.Helper()
	set := edhSet(t)
	pod := [4][]string{deck, deck, deck, deck}
	play := fourSeatSetPlay(t, set, pod, nil, nil, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= seeds; seed++ {
		if res := play(seed); res.Div != nil || res.StallUnmodelled {
			return true
		}
	}
	return false
}

// TestLandGrantFiltOrderMutation: with the prefix search taking a filter land's
// intrinsic tap before its printed filter the pod diverges (gorge's ability ask
// lists the printed filter first).
func TestLandGrantFiltOrderMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantFiltOrder = true
	defer func() { dirac.DebugLandGrantFiltOrder = false }()
	if !landGrantFiltMutated(t, landGrantFiltDeck(), 400) {
		t.Fatal("no seed diverged with the intrinsic tap searched before the printed filter")
	}
}

// TestLandGrantAltFundMutation: with a prefix search's funding tap of a bounce
// land answering its own face alone, the pod diverges (gorge's ask also offers
// the intrinsic {U}, which the search's node budget and path reach).
func TestLandGrantAltFundMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantAltFund = true
	defer func() { dirac.DebugLandGrantAltFund = false }()
	if !landGrantFiltMutated(t, append(slices.Clone(landGrantAltShared), landGrantFiltLands[:5]...), 500) {
		t.Fatal("no seed diverged with the funding tap answering the own face alone")
	}
}
