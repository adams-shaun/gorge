package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/rules"
)

// TestKernelStartRefusesTokens: the kernel models no start tokens, so a
// StartSpec that lists one must be refused up front, not dropped.
func TestKernelStartRefusesTokens(t *testing.T) {
	sp := &rules.StartSpec{Seats: make([]rules.StartSeat, 2)}
	sp.Seats[1].Tokens = []rules.StartToken{{Script: "w_1_1_soldier"}}
	if len(sp.Seats[1].Tokens) != 1 {
		t.Fatal("precondition: seat 1 lists one token")
	}
	st, err := kernelStart(sp)
	if err == nil {
		t.Fatalf("kernelStart accepted a start token (got %+v); want a refusal", st)
	}
	if !strings.Contains(err.Error(), "p1") || !strings.Contains(err.Error(), "tokens") {
		t.Fatalf("error %q does not name seat p1 and tokens", err)
	}
	// A start without tokens is still accepted.
	sp.Seats[1].Tokens = nil
	if _, err := kernelStart(sp); err != nil {
		t.Fatalf("token-free start refused: %v", err)
	}
}
