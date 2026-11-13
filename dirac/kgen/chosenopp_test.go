package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWhoChosenPlMatchesKop: build_ab.go casts a trigger's Whose to
// kop.Whose, so the Player.Chosen selector must carry kop.WhoseChosenPl's
// value (lower_chosenopp.go).
func TestWhoChosenPlMatchesKop(t *testing.T) {
	if uint8(kop.WhoseChosenPl) != whoChosenPl {
		t.Fatalf("whoChosenPl %d != kop.WhoseChosenPl %d", whoChosenPl, kop.WhoseChosenPl)
	}
}
