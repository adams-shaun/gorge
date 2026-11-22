package oracle

import (
	"strings"
	"testing"
)

// stolenLossDiesRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 5,
// pod 99): combat damage takes a player to 0 life in the same check that destroys
// their Elvish Archdruid, which an Assault Suit had handed to an opponent. CR 704.3
// performs both together, so the Archdruid dies (CR 704.5g) and Blade of the
// Bloodchief and Patron of the Vein trigger (CR 603.10a) before the loser's cards
// leave (CR 800.4a). gorge ended the loser's control effect with the loss, exiling the
// Archdruid before the batch, so no creature died and no trigger was ordered.
const stolenLossDiesRepro = "stolen_creature_dies_as_owner_loses_four_seat_pod"

// stolenLossDiesAt is the decision the recording diverged at.
const stolenLossDiesAt = 2394

func TestFourSeatStolenCreatureDiesAsOwnerLoses(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, stolenLossDiesRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= stolenLossDiesAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, stolenLossDiesAt)
	}
}
