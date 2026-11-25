package oracle

import (
	"path/filepath"
	"strings"
	"testing"
)

// haloForagerOpenSourceXRepro is a four-seat edh pod (kfuzz -kernel edh
// -seats 4, pod 68). Halo Forager's ETB "you may pay {X}" announces X while
// its controller's seven untapped mana sources include Exotic Orchard and
// Arcane Signet: an open-colour source still produces one unit per tap, so
// gorge bounds X at 7. The kernel's potential walk read the fixed any-colour
// production as whole-pool open and kept the defensive cap, 0..64.
const haloForagerOpenSourceXRepro = "halo_forager_open_source_x_bound_four_seat_pod"

// haloForagerOpenSourceXFirstAt is the decision the recording diverged at
// before the fix.
const haloForagerOpenSourceXFirstAt = 1573

// TestEdhHaloForagerOpenSourceXBoundLockstep replays the pod in lockstep past
// the recorded decision: the announcement's bound is the per-source census
// (each source once, at one tap), not trigXCap.
func TestEdhHaloForagerOpenSourceXBoundLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// The precondition the bound depends on: this is the pod whose payer holds
	// both open-colour sources beside Halo Forager, and its recorded action
	// prefix actually reaches the announcement.
	r, err := ReadRepro(filepath.Join("testdata", "repro", haloForagerOpenSourceXRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	held := map[string]bool{}
	for _, names := range append(r.Cards[:], r.MoreCards...) {
		for _, name := range names {
			held[name] = true
		}
	}
	for _, name := range []string{"Halo Forager", "Exotic Orchard", "Arcane Signet"} {
		if !held[name] {
			t.Fatalf("precondition: the recording holds no %s", name)
		}
	}
	if len(r.Actions) < haloForagerOpenSourceXFirstAt {
		t.Fatalf("precondition: %d recorded actions, want the run to reach the diverging decision %d", len(r.Actions), haloForagerOpenSourceXFirstAt)
	}
	res := fourSeatEdhReplay(t, haloForagerOpenSourceXRepro)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Errorf("the kernel stalled: %q", res.Stall)
	}
	if res.Decisions <= haloForagerOpenSourceXFirstAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, haloForagerOpenSourceXFirstAt)
	}
}
