package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPainAvailMaskBareGapsIsNotTheWindow: the bare-gaps census
// (mergeBareGaps: availGaps|availNoFilt) prices a painful source by the
// planner's rule, not the unless / ward window's. availNoFilt once shared
// availWin's bit, so a painland lost its paid-life colours and a pain-only
// source (Mana Confluence) counted with no lethal guard.
func TestPainAvailMaskBareGapsIsNotTheWindow(t *testing.T) {
	if availNoFilt&availWin != 0 || availNoFilt&availWinCol != 0 {
		t.Errorf("availNoFilt %d shares a bit with availWin %d / availWinCol %d", availNoFilt, availWin, availWinCol)
	}
	sc := newScen(t, Config{})
	painland := &pcard{manaCol: kop.ColorMask(kop.ColC), painCol: kop.ColAny, painN: 1, painLife: true}
	confluence := &pcard{painCol: kop.ColAny, painN: 1, painOnly: true, painLife: true}

	sc.s.pl[0].life = 20
	sc.s.g.availX = availPlain | availGaps | availNoFilt
	if got, want := sc.s.painAvailMask(sc.k, painland, 0), uint8(painland.manaCol)|uint8(painland.painCol)&0x3f; got != want {
		t.Errorf("painland mask %#x, want %#x (its {C} and its colours)", got, want)
	}
	sc.s.pl[0].life = 1
	if got := sc.s.painAvailMask(sc.k, confluence, 0); got != 0 {
		t.Errorf("lethal pain-only mask %#x, want 0 (the planner's guard)", got)
	}
	// the window itself still takes its own branch
	sc.s.g.availX = availWin
	if got := sc.s.painAvailMask(sc.k, confluence, 0); got == 0 {
		t.Errorf("window pain-only mask is 0, want its colours with no guard")
	}
	sc.s.g.availX = 0
}
