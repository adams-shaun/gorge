package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kwebslingEnterForms: the webSlinged cast-provenance word on a battlefield-
// entering trigger's ValidCard$ (PredWasCastEnter bit 16, composed with
// wasCast).
var kwebslingEnterForms = []formCase{
	{card: "Spiders-Man, Heroic Horde", form: "ChangesZone ValidCard$ Card.Self+wasCast+webSlinged (PredWasCastEnter PredN 16)", lir: "trigger ETB F={types=0x0 Self cmp(29 0 16  )}"},
}

// TestWebSlingedEnterWordLowers: Spiders-Man lowers with the webSlinged bit;
// the word in a spell-side filter (a SpellCast ValidCard$) or outside an
// enters trigger refuses by name, and !webSlinged stays unknown.
func TestWebSlingedEnterWordLowers(t *testing.T) {
	checkForms(t, kwebslingEnterForms)

	for _, vc := range []string{"Card.Self+wasCast+webSlinged", "Card.Self+webSlinged+wasCast", "Card.Self+webSlinged"} {
		refs, preds := kwascastLower(t, strings.Replace(kwascastTrig, "%s", vc, 1))
		if len(refs) != 0 {
			t.Errorf("%s refused: %v", vc, refs)
			continue
		}
		if n, ok := preds[kop.PredWasCastEnter]; !ok || n != wasCastWebSling {
			t.Errorf("%s lowered to preds %v, want PredWasCastEnter %d", vc, preds, wasCastWebSling)
		}
	}

	spell := "T:Mode$ SpellCast | ValidCard$ Creature.webSlinged | ValidActivatingPlayer$ You | Execute$ ProbeDraw | TriggerZones$ Battlefield\nSVar:ProbeDraw:DB$ Draw | Defined$ You | NumCards$ 1"
	if refs, _ := kwascastLower(t, spell); !hasRefusal(refs, "webSlinged on a spell") {
		t.Errorf("SpellCast ValidCard$ webSlinged: want the named spell-side refusal, got %v", refs)
	}
	static := "S:Mode$ Continuous | Affected$ Creature.YouCtrl+webSlinged | AddPower$ 1 | Description$ probe"
	if refs, _ := kwascastLower(t, static); !hasRefusal(refs, "webSlinged outside a battlefield-entering trigger") {
		t.Errorf("static Affected$ webSlinged: want the named refusal, got %v", refs)
	}
	if refs, _ := kwascastLower(t, strings.Replace(kwascastTrig, "%s", "Card.Self+!webSlinged", 1)); !hasRefusal(refs, "webSlinged") {
		t.Errorf("!webSlinged: want a refusal, got %v", refs)
	}
}
