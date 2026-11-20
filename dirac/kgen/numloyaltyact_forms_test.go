package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// numLoyaltyActForms covers the printed Mode$ NumLoyaltyAct static: Oath of
// Teferi's Twice$ over Planeswalker.YouCtrl lowers onto kop.StatNumLoyaltyAct,
// the static the kernel's loyalty-activation limit reads.
var numLoyaltyActForms = []formCase{
	{card: "Oath of Teferi", form: "printed NumLoyaltyAct over Planeswalker.YouCtrl", lir: "static NumLoyaltyAct"},
}

func TestNumLoyaltyActForms(t *testing.T) { checkForms(t, numLoyaltyActForms) }

// TestNumLoyaltyActUrzaFace lowers Urza, Planeswalker's own face: the card is
// the meld result of Urza, Lord Protector, whose two-face layout is separate
// work, so the face is lowered alone with the real script text. The static is
// the last blocker named by the ticket; the face's other abilities lower as the
// meld half does.
func TestNumLoyaltyActUrzaFace(t *testing.T) {
	cardsDir := ratchetCards(t)
	p, err := loadCardNames([]string{"Urza, Planeswalker"}, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	full := p.byName["Urza, Planeswalker"]
	var face *cards.Face
	for _, f := range full.Faces {
		if f != nil && strings.EqualFold(f.Name, "Urza, Planeswalker") {
			face = f
		}
	}
	if face == nil {
		t.Fatalf("Urza, Planeswalker has no face of that name: %d faces", len(full.Faces))
	}
	single := *full
	single.Faces = []*cards.Face{face}
	single.AlternateMode = ""
	p.byName["Urza, Planeswalker"] = &single

	lw := lowerPool(p)
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(lw.cards))
	}
	c := lw.cards[0]
	for _, r := range c.Refusals {
		t.Errorf("Urza, Planeswalker face refused: %s: %s", r.Node, r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	if !strings.Contains(sb.String(), "static NumLoyaltyAct") {
		t.Fatalf("the face's lowered IR lacks the NumLoyaltyAct static:\n%s", sb.String())
	}
}
