package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
)

// TestKernelStartRefusesFaceDown: the kernel models no face-down start
// placement, so a StartSpec that lists one must be refused up front, not
// dropped.
func TestKernelStartRefusesFaceDown(t *testing.T) {
	sp := &rules.StartSpec{Seats: make([]rules.StartSeat, 2)}
	sp.Seats[1].Battlefield = []int{3}
	sp.Seats[1].FaceDown = []rules.StartFaceDown{{Card: 3, Kind: rules.StartFaceDownManifest}}
	if len(sp.Seats[1].FaceDown) != 1 {
		t.Fatal("precondition: seat 1 lists one face-down entry")
	}
	st, err := kernelStart(sp)
	if err == nil {
		t.Fatalf("kernelStart accepted a face-down start entry (got %+v); want a refusal", st)
	}
	if !strings.Contains(err.Error(), "p1") || !strings.Contains(err.Error(), "face-down") {
		t.Fatalf("error %q does not name seat p1 and face-down", err)
	}
	// A start without face-down entries is still accepted.
	sp.Seats[1].FaceDown = nil
	if _, err := kernelStart(sp); err != nil {
		t.Fatalf("face-down-free start refused: %v", err)
	}
}
