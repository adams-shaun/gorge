package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// destroyAllUmbraRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4
// -seed 3, pod 288): Phyrexian Rebirth ("destroy all creatures, then create an
// X/X Horror, X the number of creatures destroyed this way") with a Snake Umbra
// on a creature. The Umbra armor destroys the Aura instead of its bearer (CR
// 702.89a), so the bearer is not destroyed and the Aura is no creature: X
// counts neither. The kernel remembered the Aura, so X (and the Horror) was one
// too large.
const destroyAllUmbraRepro = "destroy_all_remember_skips_umbra_aura_four_seat_pod"

// destroyAllUmbraAt is the decision the recording diverged at.
const destroyAllUmbraAt = 1109

func TestFourSeatDestroyAllRememberSkipsUmbraAura(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, destroyAllUmbraRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= destroyAllUmbraAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, destroyAllUmbraAt)
	}
}

// TestFourSeatDestroyAllRememberSkipsUmbraAuraMutation proves the replay
// reaches the umbra answer: remembering the Aura (the old behaviour) diverges.
func TestFourSeatDestroyAllRememberSkipsUmbraAuraMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRememberUmbraAura = true
	defer func() { dirac.DebugRememberUmbraAura = false }()
	if res := fourSeatEdhReplay(t, destroyAllUmbraRepro); res.Div == nil {
		t.Fatal("remembering the Aura changed no game: the replay does not reach the umbra answer")
	}
}
