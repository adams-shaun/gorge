package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestEvalSacrificedHeads pins the Sacrificed$<Property> resolution added for
// the Ghoulcaller Gisa bug (Task sac1): a Sacrificed$... body answers a
// characteristic of the object(s) this resolving spell/ability sacrificed,
// read from the last-known-information snapshot captured at the instant of
// the sacrifice (state.SacrificedInfo), never from the (now-graveyard, layer-
// and-counter-stripped) live object. The four in-scope heads are CardPower,
// CardToughness, CardManaCost and Amount (the count of objects sacrificed);
// the /Op suffix applies the same arithmetic as Count$'s.
func TestEvalSacrificedHeads(t *testing.T) {
	h := newHost(t, 2)
	// Two sacrificed permanents whose LKI we control directly: a 1-power
	// 3-mana-value artifact and a 4-power 2-mana-value creature.
	c := &Ctx{
		Sacrificed: []state.SacrificedInfo{
			{Obj: 1, Power: 1, Toughness: 1, ManaValue: 3},
			{Obj: 2, Power: 4, Toughness: 5, ManaValue: 2},
		},
	}
	cases := []struct {
		expr string
		want int32
	}{
		{"Sacrificed$CardPower", 5},
		{"Sacrificed$CardToughness", 6},
		{"Sacrificed$CardManaCost", 5},
		{"Sacrificed$Amount", 2},
		{"Sacrificed$Amount/Plus.1", 3},
		{"Sacrificed$CardPower/Minus1", 4},
		{"Sacrificed$CardPower/Times.2", 10},
		{"Sacrificed$CardPower/Twice", 10},
	}
	for _, tc := range cases {
		if got := EvalCount(h, c, tc.expr); got != tc.want {
			t.Errorf("%q = %d, want %d", tc.expr, got, tc.want)
		}
	}
}

// TestEvalSacrificedSingleEqualsSum: the corpus Sac cost almost always has
// exactly one candidate, so CardPower/Amount for a single-object sacrifice is
// that object's own value -- the Gisa shape. This pins the single path
// directly (sum == the one value) so a future change that breaks the common
// case is caught even though TestEvalSacrificedHeads exercises the multi case.
func TestEvalSacrificedSingleEqualsSum(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Sacrificed: []state.SacrificedInfo{{Obj: 9, Power: 4, Toughness: 4, ManaValue: 2}}}
	if got := EvalCount(h, c, "Sacrificed$CardPower"); got != 4 {
		t.Fatalf("CardPower = %d, want 4", got)
	}
	if got := EvalCount(h, c, "Sacrificed$Amount"); got != 1 {
		t.Fatalf("Amount = %d, want 1", got)
	}
}

// TestEvalSacrificedEmptyAndUnknownDegradeToZero: nothing captured (or an
// out-of-scope head) resolves to zero, the package's "the card did nothing"
// totality convention, so Sacrificed$Valid/CardTypes/ChromaSource/... stay
// silent and never panic the resolution.
func TestEvalSacrificedEmptyAndUnknownDegradeToZero(t *testing.T) {
	h := newHost(t, 2)
	empty := &Ctx{}
	if got := EvalCount(h, empty, "Sacrificed$CardPower"); got != 0 {
		t.Fatalf("empty Sacrificed$CardPower = %d, want 0", got)
	}
	if got := EvalCount(h, empty, "Sacrificed$Amount"); got != 0 {
		t.Fatalf("empty Sacrificed$Amount = %d, want 0", got)
	}
	// An out-of-scope head still falls through to 0 (same as before the fix).
	full := &Ctx{Sacrificed: []state.SacrificedInfo{{Obj: 1, Power: 4, Toughness: 4, ManaValue: 2}}}
	if got := EvalCount(h, full, "Sacrificed$CardTypes"); got != 0 {
		t.Fatalf("Sacrificed$CardTypes = %d, want 0 (out of scope)", got)
	}
	if got := EvalCount(h, full, "Sacrificed$CardNumColors"); got != 0 {
		t.Fatalf("Sacrificed$CardNumColors = %d, want 0 (out of scope)", got)
	}
}

// TestNumSvarIndirectionResolvesSacrificed pins the full Num path Gisa uses:
// TokenAmount$ X, where SVar:X is "Sacrificed$CardPower". Num resolves the
// non-literal "X" through the SVar table into EvalCount, which must read the
// captured LKI -- not the zero the old fall-through returned.
func TestNumSvarIndirectionResolvesSacrificed(t *testing.T) {
	h := newHost(t, 2)
	sa := &cards.SA{Kind: "AB", API: "Token", Params: map[string]string{"TokenAmount": "X"}}
	c := &Ctx{
		SVars: map[string]string{"X": "Sacrificed$CardPower"},
		Sacrificed: []state.SacrificedInfo{
			{Obj: 1, Power: 4, Toughness: 4, ManaValue: 2},
		},
	}
	if got := Num(h, c, sa, "TokenAmount", 1); got != 4 {
		t.Fatalf("Num TokenAmount = %d, want 4", got)
	}
}
