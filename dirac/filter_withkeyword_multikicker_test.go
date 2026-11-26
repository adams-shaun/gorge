package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPrintedKeywordMultikickerIsNotSquadOrReplicate: kgen shares
// CardDef.HasMultikicker with Squad and Replicate (their per-payment mana
// half), but only K:Multikicker prints the keyword, so withKeyword Multikicker
// reads false on the other two (Mimeofacture, tail-21 of the oracle atom diff).
func TestPrintedKeywordMultikickerIsNotSquadOrReplicate(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind uint8
		want bool
	}{
		{"multikicker", kop.RepeatMultikicker, true},
		{"squad", kop.RepeatSquad, false},
		{"replicate", kop.RepeatReplicate, false},
	} {
		cd := &kop.CardDef{HasMultikicker: true, RepeatedKind: tc.kind}
		if got := printedKeyword(cd, kop.KwMultikicker); got != tc.want {
			t.Errorf("%s: printedKeyword(Multikicker) = %v, want %v", tc.name, got, tc.want)
		}
	}
	if printedKeyword(&kop.CardDef{}, kop.KwMultikicker) {
		t.Error("a card with no repeated cost prints Multikicker")
	}
}
