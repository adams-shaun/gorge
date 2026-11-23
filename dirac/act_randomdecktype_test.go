package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRandomDeckTypePrintedSubs: the random creature-type pick reads a card's
// printed subtypes (gorge Face().Types), so a changeling's kgen-expanded roster
// never widens the option list or the no-ask fallback.
func TestRandomDeckTypePrintedSubs(t *testing.T) {
	var printed, roster kop.SubMask
	printed[0] = 1 << 3
	roster[0] = 1<<3 | 1<<5 | 1<<7
	plain := &kop.CardDef{Subtypes: roster}
	if got := printedCreatureSubs(plain); got != roster {
		t.Fatalf("plain card: got %v, want its subtypes %v", got, roster)
	}
	ch := &kop.CardDef{Subtypes: roster, PrintedSubtypes: printed, AllCreatureTypes: true}
	if got := printedCreatureSubs(ch); got != printed {
		t.Fatalf("changeling: got %v, want its printed subtypes %v", got, printed)
	}
}
