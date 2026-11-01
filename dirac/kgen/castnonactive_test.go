package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWhoNonActiveMatchesKop: build_ab.go casts the kgen who value to
// kop.Whose, so the two constants must agree.
func TestWhoNonActiveMatchesKop(t *testing.T) {
	if uint8(kop.WhoseNonActive) != whoNonActive {
		t.Fatalf("whoNonActive %d != kop.WhoseNonActive %d", whoNonActive, kop.WhoseNonActive)
	}
}
