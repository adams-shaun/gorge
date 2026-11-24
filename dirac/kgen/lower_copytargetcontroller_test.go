package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestLowerCopypermanentNonTargetedController pins Fractured Identity's new
// script: the ChangeZone exile's SubAbility copies the targeted permanent for
// each player other than its controller. The lowering reads Controller$
// NonTargetedController as the same CopyNonRemCtrl walk over the TARGETED
// source (not only a Defined$ Remembered one).
func TestLowerCopypermanentNonTargetedController(t *testing.T) {
	src := "Name:Copier\nManaCost:B\nTypes:Sorcery\n" +
		"A:SP$ ChangeZone | ValidTgts$ Permanent.nonLand | Origin$ Battlefield | Destination$ Exile | SubAbility$ PrbCopy\n" +
		"SVar:PrbCopy:DB$ CopyPermanent | Controller$ NonTargetedController | Defined$ Targeted\n"
	card := cnCard(t, src)
	p := &pool{names: []string{"Copier"}, byName: map[string]*cards.Card{"Copier": card}, nDecks: map[string]int{}, reg: cards.NewRegistry()}
	lc := lowerCard(p, "Copier", card, false)
	if r := lowerRefusals(lc); len(r) != 0 {
		t.Fatalf("refused: %v", r)
	}
	var ir strings.Builder
	dumpLIR(&ir, lc)
	for _, want := range []string{"CopyAsToken", "obj=Target", "CS=NonRemCtrl"} {
		if !strings.Contains(ir.String(), want) {
			t.Fatalf("lowered IR lacks %q:\n%s", want, ir.String())
		}
	}

	// The selector is the targeted spelling only: the remembered spelling is
	// a different subject, and a mismatch stays a named refusal rather than
	// silently copying the wrong object.
	c2, _ := cnLower(t, "SP$ CopyPermanent | Cost$ B | Defined$ Remembered | Controller$ NonTargetedController")
	got := lowerRefusals(c2)
	if len(got) == 0 || !strings.Contains(got[0].Reason, "CopyPermanent Controller$ NonTargetedController over") {
		t.Fatalf("refusals = %v, want the NonTargetedController subject refusal", got)
	}
}
