package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDigUntilOptionalShuffleDeclineCoversShuffle pins the decline reach of
// an Optional$ DigUntil whose Shuffle$ True emits a Shuffle op behind the dig
// (Thicket Elemental's kicked ETB). gorge's effDigUntil declines with
// continue before the walk's shuffle tail (engine/effects/diguntil.go:208),
// so answering "no" — and an empty library's unasked guard — must skip the
// Shuffle as well as the DigUntil: a declined dig neither reorders the
// library nor draws RNG. The guard's skip therefore covers both instructions
// once the shuffle is emitted; a skip of 1 leaves the Shuffle behind the
// decline and diverges (the reviewer's break attempt).
func TestDigUntilOptionalShuffleDeclineCoversShuffle(t *testing.T) {
	c := lowerCorpusCard(t, "Thicket Elemental")
	if r := refusalText(c); r != "" {
		t.Fatalf("Thicket Elemental: refused: %s", r)
	}
	found := false
	for ai, ab := range c.Abilities {
		for i, in := range ab.Prog {
			if in.Op != "MayAsk" || in.C != int(kop.MayGuardDigUntil) {
				continue
			}
			found = true
			if i+2 >= len(ab.Prog) || ab.Prog[i+1].Op != "DigUntil" || ab.Prog[i+2].Op != "Shuffle" {
				t.Fatalf("ability %d: the MayGuardDigUntil guard at %d is not followed by DigUntil + Shuffle (prog %v)", ai, i, progOps(c))
			}
			if in.Skip != 2 {
				t.Errorf("ability %d: MayAsk skip %d, want 2 (the decline covers the DigUntil and the Shuffle; gorge continues before the walk's shuffle tail)", ai, in.Skip)
			}
		}
	}
	if !found {
		t.Fatal("Thicket Elemental: no MayGuardDigUntil guard lowered (the card shape moved?)")
	}
}
