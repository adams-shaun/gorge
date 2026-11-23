package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// A commander-identity land under Stormtide Leviathan (CR 305.6, 605.3b, 903.4):
// Command Tower makes a colour of its controller's commander identity, and the
// granted Island adds the intrinsic {U} beside it (ColorIdentity is no single
// colour, so the intrinsic ability is never deduplicated). gorge ranks the
// source over the union, three types for a White-Green commander, below a
// Forest's {G} and {U}; the kernel's identity-only view tied the two and took
// the lower object id.
var landGrantIdentLands = []string{"Command Tower"}

func landGrantIdentDeck() []string {
	d := slices.Clone(landGrantAltShared)
	d = append(d, "Command Tower", "Command Tower", "Command Tower", "Forest", "Forest", "Shuri's Fabricator", "Trostani, Selesnya's Voice")
	return d
}

func landGrantIdentSrc(name string, _ int) bool { return slices.Contains(landGrantIdentLands, name) }

func landGrantIdentRun(t *testing.T, seeds uint64, quiet bool) (reached, bad int) {
	t.Helper()
	set := edhSet(t)
	check := func(g *GorgeSide, _ Side) *Divergence {
		if landGrantMeets(g, landGrantIdentSrc) {
			reached++
		}
		return nil
	}
	deck := landGrantIdentDeck()
	pod := [4][]string{deck, deck, deck, deck}
	cmd := []int{len(deck) - 1, len(deck) - 1, len(deck) - 1, len(deck) - 1}
	play := fourSeatSetPlayCmd(t, set, pod, cmd, nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= seeds; seed++ {
		res := play(seed)
		if res.Div != nil {
			bad++
			d := res.Div
			if !quiet {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
			}
		}
		if res.StallUnmodelled || strings.Contains(res.Stall, "landgrant") {
			bad++
			if !quiet {
				t.Errorf("seed %d: kernel stopped: %s", seed, res.Stall)
			}
		}
	}
	return reached, bad
}

// TestFourSeatLandGrantIdentLockstep: no seed diverges or stalls, and the
// Leviathan meets an untapped Command Tower.
func TestFourSeatLandGrantIdentLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reached, _ := landGrantIdentRun(t, 60, false)
	if reached == 0 {
		t.Fatal("the Leviathan never met an untapped Command Tower")
	}
	t.Logf("%d decision points with the Leviathan beside an untapped Command Tower", reached)
}

// TestLandGrantIdentViewMutation: with the identity view keeping its own colours
// alone the pod diverges (gorge ranks the tower over the intrinsic {U} too).
func TestLandGrantIdentViewMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLandGrantIdentOff = true
	defer func() { dirac.DebugLandGrantIdentOff = false }()
	if _, bad := landGrantIdentRun(t, 60, true); bad == 0 {
		t.Fatal("no seed diverged with the identity view dropping the intrinsic colour")
	}
}

const landGrantIdentPod = "landgrant_ident_exile_order_pod"

// landGrantIdentPodAt is the last decision the recording diverged at before the
// two fixes: the tiebreak (1883, Shuri's Fabricator cast beside a Command Tower
// under the Leviathan) and the leave-game exile order (2819).
const landGrantIdentPodAt = 2819

// TestLandGrantIdentPodLockstep replays the four-seat edh pod (seed 2 of the
// 100-card sweep) past both recorded decisions.
func TestLandGrantIdentPodLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, landGrantIdentPod)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= landGrantIdentPodAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, landGrantIdentPodAt)
	}
}

// TestLandGrantIdentPodMutations: each fix's switch makes the pod diverge, at or
// before the decision it recorded.
func TestLandGrantIdentPodMutations(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range []struct {
		name string
		set  func(bool)
		at   int
	}{
		{"identity view", func(b bool) { dirac.DebugLandGrantIdentOff = b }, 1883},
		{"exile order", func(b bool) { dirac.DebugLeaveExileSlotOrder = b }, landGrantIdentPodAt},
	} {
		c.set(true)
		res := fourSeatEdhReplay(t, landGrantIdentPod)
		c.set(false)
		if res.Div == nil {
			t.Errorf("%s: no divergence with the fix off", c.name)
		} else if res.Div.At > c.at {
			t.Errorf("%s: diverged at %d, after the recorded decision %d", c.name, res.Div.At, c.at)
		}
	}
}
