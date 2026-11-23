package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// multikickGrantRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed
// 5, pod 128): Marshal's Anthem cast multikicked with Chromatic Lantern's
// granted "{T}: Add one mana of any color" among the lands. gorge's composed
// kicked play (the base plus one multikicker payment, CR 702.43) is priced by
// its granted-mana prefix, which floats the lands' mana before the cast; the
// kernel priced the play at the base alone (and refused the Multikicker shape),
// tapped nothing, and its witness found no plan.
const multikickGrantRepro = "multikicker_grant_prefix_prices_one_payment_four_seat_pod"

// multikickGrantAt is the decision the recording panicked at.
const multikickGrantAt = 2202

func TestFourSeatMultikickerGrantPrefix(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, multikickGrantRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= multikickGrantAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, multikickGrantAt)
	}
}

// TestFourSeatMultikickerGrantPrefixMutation proves the replay reaches the
// prefix: pricing the base alone (the old behaviour) diverges.
func TestFourSeatMultikickerGrantPrefixMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugGrantKickNoMulti = true
	defer func() { dirac.DebugGrantKickNoMulti = false }()
	if res := fourSeatEdhReplay(t, multikickGrantRepro); res.Div == nil {
		t.Fatal("pricing the base alone changed no game: the replay does not reach the prefix")
	}
}
