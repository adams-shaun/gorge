package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapReflectAnyColourIsOneColour: Mana Flare's body over a plain any-colour
// source makes two mana of the ONE colour it picks (CR 106.12a), so {W}{G} is
// not payable from it alone while {G}{G} is.
func TestTapReflectAnyColourIsOneColour(t *testing.T) {
	sc, _ := tapReflectScen(t, nil)
	var a avail
	sc.s.availFor(sc.k, 0, &a)
	if a.total != 2 {
		t.Fatalf("precondition: total = %d, want 2 (one any-colour tap plus its reflected extra)", a.total)
	}
	var wg, gg kop.ManaCost
	wg.Pips[kop.W], wg.Pips[kop.G] = 1, 1
	gg.Pips[kop.G] = 2
	if !a.affords(&gg, pipMaskOf(&gg), 0) {
		t.Fatalf("{G}{G} not affordable (total=%d nOne=%d)", a.total, a.nOne)
	}
	if a.affords(&wg, pipMaskOf(&wg), 0) {
		t.Fatalf("{W}{G} affordable from one any-colour tap (total=%d nOne=%d)", a.total, a.nOne)
	}
}
