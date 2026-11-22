package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/state"
)

// A creature land under Ashaya (CR 305.6, 613.1d) holds the Forest's intrinsic
// {G} as its own ability, and gorge's planner counts it as the source's own unit;
// a granted ability beside it (Citanul Hierophants' "{T}: Add {G}") is not the
// source's only one. Arixmethes adds its own "{T}: Add {G}{U}" (all at once)
// beside the intrinsic {G}: the own-vs-intrinsic pair.

// landGrantGrantedReach counts decision points where one player controls
// Ashaya, Citanul Hierophants and Arixmethes.
func landGrantGrantedReach(g *GorgeSide) bool {
	for p := range 4 {
		var a, h, x bool
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
				switch o.Face().Name {
				case "Ashaya, Soul of the Wild":
					a = true
				case "Citanul Hierophants":
					h = true
				case "Arixmethes, Slumbering Isle":
					x = true
				}
			}
		}
		if a && h && x {
			return true
		}
	}
	return false
}

func landGrantGrantedRun(t *testing.T, seeds uint64, quiet bool) (reached, bad int) {
	t.Helper()
	set := edhSet(t)
	check := func(g *GorgeSide, _ Side) *Divergence {
		if landGrantGrantedReach(g) {
			reached++
		}
		return nil
	}
	deck := slices.Clone(landGrantPairDeck)
	pod := [4][]string{deck, deck, deck, deck}
	play := fourSeatSetPlay(t, set, pod, nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil || res.StallUnmodelled || res.Stall == "kernel landgrant" {
			bad++
			if !quiet {
				t.Errorf("seed %d: div %v stall %q", seed, res.Div, res.Stall)
			}
		}
	}
	return reached, bad
}

// TestFourSeatLandGrantGrantedLockstep: no seed diverges or stalls, and Ashaya,
// the Hierophants and Arixmethes meet under one controller.
func TestFourSeatLandGrantGrantedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantGrantedRun(t, 40, false)
	if reached == 0 {
		t.Fatal("Ashaya, the Hierophants and Arixmethes never met")
	}
	t.Logf("%d decision points", reached)
}

// TestLandGrantGrantedMutation: with the intrinsic ability no own one beside a
// granted ability, a seed stalls or diverges.
func TestLandGrantGrantedMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantOwnOff = true
	dirac.DebugLandGrantNoPairGuard = true
	defer func() { dirac.DebugLandGrantOwnOff, dirac.DebugLandGrantNoPairGuard = false, false }()
	if _, bad := landGrantGrantedRun(t, 40, true); bad == 0 {
		t.Fatal("no seed stalled or diverged with the intrinsic ability no own one")
	}
}

// TestLandGrantArixmethesOffRefused: with the pair views off, Arixmethes beside
// Ashaya is refused by name.
func TestLandGrantArixmethesOffRefused(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantAltOff = true
	defer func() { dirac.DebugLandGrantAltOff = false }()
	landGrantRefusedNames(t, landGrantPairDeck, []string{"Ashaya, Soul of the Wild", "Arixmethes, Slumbering Isle"}, []string{"Citanul Hierophants"})
}
