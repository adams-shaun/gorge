package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoTutorPickShapes: Fertilid and Intuition (and Gifts Ungiven, the
// same pick) lower with no refusal. Fertilid's "target player searches their
// library" has no DefinedPlayer$ (gorge searchPlayersFor reads the owners off
// the targets, Chooser$ Targeted answering); Intuition's and Gifts Ungiven's
// second link is the targeted opponent's pick among the remembered cards of
// your library (SrChooserOpp), followed by a sweep of the rest that shuffles
// only when a swept card left the library (gorge changeZoneAllShuffleTail).
func TestDemoTutorPickShapes(t *testing.T) {
	names := []string{"Fertilid", "Intuition", "Gifts Ungiven"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	oppPicks := map[string]bool{}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		for _, ab := range b.def.Abilities {
			for _, in := range ab.Prog {
				if in.Op == kop.OpSearchEx && in.B.N&kop.SrChooserOpp != 0 {
					oppPicks[b.def.Name] = true
				}
			}
		}
	}
	for _, n := range []string{"Intuition", "Gifts Ungiven"} {
		if !oppPicks[n] {
			t.Errorf("%s: no search answered by the opponent (SrChooserOpp)", n)
		}
	}
}
