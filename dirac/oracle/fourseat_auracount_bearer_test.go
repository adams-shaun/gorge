package oracle

import (
	"strings"
	"testing"
)

// auraCountBearerRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 5,
// pod 68): Sage's Reverie on Puppeteer Clique counts "Aura you control that's
// attached to a creature". Combat damage kills Ellivere of the Wild Court, whose
// Monster Role waits for its CR 704.5m sweep; gorge still counted that Role (its
// bearer in the graveyard) and showed the Clique 7/6, the kernel 6/5. A card in a
// graveyard is no creature (CR 110.1, 205.2a), so gorge's AttachedTo filter now
// requires the bearer on the battlefield.
const auraCountBearerRepro = "aura_count_bearer_in_graveyard_four_seat_pod"

// auraCountBearerAt is the decision the recording diverged at.
const auraCountBearerAt = 3507

func TestFourSeatAuraCountBearerGone(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, auraCountBearerRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= auraCountBearerAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, auraCountBearerAt)
	}
}
