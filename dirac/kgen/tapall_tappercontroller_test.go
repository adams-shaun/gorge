package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapAllTapperControllerBuilt: Power Sink's decline taps the lands with
// TapperController$ True, so each land's own controller is the tapper (gorge
// effTapAll). The lowering sets C 1 and the build must carry it onto the
// OpTapAll instruction, or the kernel taps them as the resolving controller.
func TestTapAllTapperControllerBuilt(t *testing.T) {
	c := lowerCorpusCard(t, "Power Sink")
	for _, rf := range c.Refusals {
		t.Fatalf("Power Sink refused: %s", rf.Reason)
	}
	pt := newPoolTables(&lowered{cards: []*lCard{c}}, nil)
	b := buildCard(pt, c)
	if len(b.refusals) != 0 {
		t.Fatalf("Power Sink: build refused: %v", b.refusals)
	}
	n := 0
	for _, a := range b.def.Abilities {
		for _, in := range a.Prog {
			if in.Op != kop.OpTapAll {
				continue
			}
			n++
			if in.C != 1 {
				t.Errorf("OpTapAll C = %d, want 1 (TapperController$ True)", in.C)
			}
		}
	}
	if n != 1 {
		t.Fatalf("Power Sink: %d OpTapAll instructions, want 1", n)
	}
}
