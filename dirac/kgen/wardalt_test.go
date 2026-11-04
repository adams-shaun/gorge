package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A ward of discarding a card or paying {N} lowers to WardDiscOrN = N, not to
// a plain discard ward or a plain {N} ward.
func TestWardDiscardOrPayLowers(t *testing.T) {
	const titania = "Titania, Rugged Rumbler"
	kp := kfhBuild(t, []string{titania}, nil)
	if rs := kp.cards[titania].Refusals; len(rs) != 0 {
		t.Fatalf("%s refused: %v", titania, rs)
	}
	d := kp.built[titania].def
	if d.WardDiscOrN != 2 || d.Keywords&kop.KwWard == 0 || d.WardDiscard != 0 || d.WardN != 0 {
		t.Errorf("%s: WardDiscOrN %d KwWard %v WardDiscard %d WardN %d", titania, d.WardDiscOrN, d.Keywords&kop.KwWard != 0, d.WardDiscard, d.WardN)
	}
}
