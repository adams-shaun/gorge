package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// leaveReturnRepros are four-seat edh pods (kfuzz -kernel edh -seats 4, seed 4
// pair 2 and seed 3 pair 28): a seat leaves the game holding a Malfunctioning
// Holodeck that exiled another seat's Door of Destinies (Herald's Horn) until
// it left. The Holodeck ceases (CR 800.4a), so the exiled card returns under
// its owner (CR 610.3) and gorge asks its as-enters "choose a creature type"
// with the card still in exile; the kernel entered it with no choice.
var leaveReturnRepros = map[string]int{
	"leaving_exiler_owner_returns_choose_card_door_of_destinies_four_seat_pod": 2534,
	"leaving_exiler_owner_returns_choose_card_heralds_horn_four_seat_pod":      2824,
}

func TestFourSeatLeaveGameReturnAsksChoice(t *testing.T) {
	if isolate(t) {
		return
	}
	for name, at := range leaveReturnRepros {
		res := fourSeatEdhReplay(t, name)
		if res.Div != nil {
			t.Fatalf("%s: divergence at %d: %s %s %v", name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= at {
			t.Fatalf("%s: stall %q after %d decisions, want a clean game past %d", name, res.Stall, res.Decisions, at)
		}
	}
}

// TestFourSeatLeaveGameReturnAsksChoiceMutation proves the replays reach the
// return: parking the choice only for a cost's sacrifice diverges.
func TestFourSeatLeaveGameReturnAsksChoiceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugReturnLeaveNoAsk = true
	defer func() { dirac.DebugReturnLeaveNoAsk = false }()
	for name := range leaveReturnRepros {
		if res := fourSeatEdhReplay(t, name); res.Div == nil {
			t.Fatalf("%s: the old parking changed no game: the replay does not reach the return", name)
		}
	}
}
