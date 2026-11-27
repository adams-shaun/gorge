package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoTokenscriptListShapes: Academy Manufactor (a CreateToken rewriter
// with a comma-listed TokenScript$ and a ValidToken$ filter) and Overencumbered
// (the same list created for TokenOwner$ EnchantedPlayer, a Phase trigger over
// ValidPlayer$ Opponent.EnchantedBy, and an Effect "creatures can't attack"
// Duration$ UntilEndOfCombat) lower with no refusal, and build to the shapes
// the kernel reads.
func TestDemoTokenscriptListShapes(t *testing.T) {
	names := []string{"Academy Manufactor", "Overencumbered"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	var list, enchOwner, enchOpp, eocEmblem bool
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
		for _, ab := range b.def.Abilities {
			if ab.Kind == kop.AbReplacement && ab.Repl.Kind == kop.ReplRewriteTokens && len(ab.Prog) == 2 && ab.Repl.Filter != 0 {
				list = true
			}
			if ab.Kind == kop.AbTriggered && ab.Trig.Whose == kop.WhoseEnchantedOpp {
				enchOpp = true
			}
			for _, in := range ab.Prog {
				if in.Op == kop.OpCreateToken && in.Obj == kop.RefEnchantedPlayer && in.B.N&kop.TrList != 0 {
					enchOwner = true
				}
			}
		}
	}
	for _, e := range pt.emblems {
		if e.UntilEndCombat && e.UntilEOT {
			eocEmblem = true
		}
	}
	if !list {
		t.Error("Academy Manufactor: no filtered list rewriter with two further cards")
	}
	if !enchOwner {
		t.Error("Overencumbered: no listed CreateToken over the enchanted player")
	}
	if !enchOpp {
		t.Error("Overencumbered: no Phase trigger with Whose WhoseEnchantedOpp")
	}
	if !eocEmblem {
		t.Error("Overencumbered: no until-end-of-combat emblem")
	}
}
