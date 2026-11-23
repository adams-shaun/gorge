package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// exileXWindowRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 5,
// pod 20): Skeletal Scrying cast from exile (Outpost Siege) with X = 5 over five
// floating mana and an untapped Boros Signet. gorge pays the announced {X}
// through its CR 601.2g window before it asks the exile-X graveyard pick
// (manaWindowAsk before costObjectAsks), so the Signet is tapped when the pick is
// posed; the kernel's lowered cast paid at the payment, after the pick.
const exileXWindowRepro = "exile_x_window_taps_signet_before_pick_four_seat_pod"

// exileXWindowAt is the decision the recording diverged at.
const exileXWindowAt = 1950

func TestFourSeatExileXWindowFirst(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, exileXWindowRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= exileXWindowAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, exileXWindowAt)
	}
}

// TestFourSeatExileXWindowFirstMutation proves the replay reaches the pick:
// paying after it (the old order) diverges.
func TestFourSeatExileXWindowFirstMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugExileXWindowLate = true
	defer func() { dirac.DebugExileXWindowLate = false }()
	if res := fourSeatEdhReplay(t, exileXWindowRepro); res.Div == nil {
		t.Fatal("paying after the pick changed no game: the replay does not reach it")
	}
}
