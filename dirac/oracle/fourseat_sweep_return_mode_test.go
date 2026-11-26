package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Two four-seat cmd5 pods (kfuzz -kernel cmd5 -seats 4 -mulligans 1, pairs 830
// seed 1 autopay/heuristic and 681 seed 2 full/random) where seat 1 loses to
// combat damage holding a Grasp of Fate that exiled seat 3's Outpost Siege
// until it leaves. The seat leaving ceases the Grasp (CR 800.4a), the Siege
// returns outside any resolution, and its owner chooses Khans or Dragons with
// the card still in exile (CR 614.12, 610.3). gorge answered that ask with its
// default (no tape run served it) while the kernel posed it: gorge now poses it
// from the entry boundary (engine/rules/etb_modechoice.go) and the kernel parks
// the returning Siege's mode ask in exile (dirac/face_siegemode.go).
var sweepReturnModeRepros = map[string]int{
	"cmd5_four_seat_sweep_return_siege_mode_830": 2368,
	"cmd5_four_seat_sweep_return_siege_mode_681": 2699,
}

func TestFourSeatSweepReturnAsksSiegeMode(t *testing.T) {
	if isolate(t) {
		return
	}
	for name, at := range sweepReturnModeRepros {
		res := fourSeatCmd5Replay(t, name)
		if res.Div != nil {
			t.Fatalf("%s: divergence at %d: %s %s %v", name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= at {
			t.Fatalf("%s: stall %q after %d decisions, want a clean game past %d", name, res.Stall, res.Decisions, at)
		}
	}
}

// TestFourSeatSweepReturnAsksSiegeModeMutation proves each replay reaches the
// return: gorge defaulting the ask, and the kernel returning the Siege before
// asking, each diverge.
func TestFourSeatSweepReturnAsksSiegeModeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	defer func() { rules.DebugETBModeNoEntryAsk, dirac.DebugReturnModeNoPark = false, false }()
	for name, flip := range map[string]*bool{
		"cmd5_four_seat_sweep_return_siege_mode_830": &rules.DebugETBModeNoEntryAsk,
		"cmd5_four_seat_sweep_return_siege_mode_681": &dirac.DebugReturnModeNoPark,
	} {
		*flip = true
		res := fourSeatCmd5Replay(t, name)
		*flip = false
		if res.Div == nil {
			t.Fatalf("%s: the old behaviour changed no game: the replay does not reach the return", name)
		}
	}
}
