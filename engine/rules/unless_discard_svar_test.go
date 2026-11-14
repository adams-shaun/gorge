package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestUnlessDiscardSVarCountFolds pins Court of Ambition's "each opponent
// loses 3 life unless they discard a card; if you're the monarch, instead
// ... loses 6 life unless they discard two cards": its UnlessCost$
// Discard<Y/Card> names SVar:Y = Count$Monarch.2.1, which the resolution
// folds into the token (effects.UnlessCostResolved) so the strict parser
// prices one or two discards. Raw, the token is unpriceable and the payer
// could only decline (CR 118.12: the opponent may discard instead).
func TestUnlessDiscardSVarCountFolds(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	court := mustCorpusCard(t, reg, "Court of Ambition")
	sa := unlessSVarSA(t, court, "Discard<Y/Card>")
	if _, ok := ParseUnlessCost("Discard<Y/Card>"); ok {
		t.Fatal("precondition: the raw Discard<Y/Card> token must be unpriceable")
	}
	e := handEngine(t, court)
	courtID := onBoard(t, e, 0, "Name:Court of Ambition\nManaCost:2 B B\nTypes:Enchantment\nOracle:x\n")
	ctx := &effects.Ctx{Source: courtID, Controller: 0, SVars: court.Faces[0].SVars}
	for _, tc := range []struct {
		monarch bool
		want    string
		n       int32
	}{{false, "Discard<1/Card>", 1}, {true, "Discard<2/Card>", 2}} {
		e.G.HasMonarch, e.G.Monarch = tc.monarch, 0
		got := effects.UnlessCostResolved(e, ctx, sa)
		if got != tc.want {
			t.Fatalf("monarch=%v: UnlessCostResolved = %q, want %q", tc.monarch, got, tc.want)
		}
		c, ok := ParseUnlessCost(got)
		if !ok || len(c.Discard) != 1 || c.Discard[0].N != tc.n || c.Discard[0].Spec != "Card" {
			t.Fatalf("monarch=%v: parsed %q = %+v ok=%v, want one Discard of %d cards", tc.monarch, got, c, ok, tc.n)
		}
	}
}
