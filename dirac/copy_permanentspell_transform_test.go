package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPermcopyTransformFrontResolves: the copy of a transforming card's
// permanent spell cast front face up resolves as a token of the same card
// (gorge's StackCopy keeps Card and FaceIdx, CR 707.10g); the game does not
// stall.
func TestPermcopyTransformFrontResolves(t *testing.T) {
	tb, _, f, w, _ := permcopyTable()
	tb.Cards[f].FaceMode = kop.FaceTransform
	sc, _, c, _ := permcopyCastCard(t, tb, f, w)
	sc.resolveAll(t)
	if r := sc.s.StallReason(); r != 0 {
		t.Fatalf("stalled at %q", StallName(r))
	}
	sl := sc.copiesOf(c)
	if len(sl) != 2 {
		t.Fatalf("%d permanents of the card, want the original and its copy", len(sl))
	}
	tokens := 0
	for _, s := range sl {
		if of := sc.s.objs[sc.s.perms[s].obj].of; of&ofToken != 0 && of&ofCopy == 0 {
			tokens++
		}
	}
	if tokens != 1 {
		t.Fatalf("%d token permanents, want the copy's one", tokens)
	}
}
