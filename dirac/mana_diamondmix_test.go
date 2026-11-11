package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDiamondMixStallClassIsUnmodelled: the Diamond-beside-another-gap
// refusal is a rule gap, not a capacity limit, and is named diamondmix.
func TestDiamondMixStallClassIsUnmodelled(t *testing.T) {
	if got := StallClassOf(StallDiamondMix); got != StallUnmodelled {
		t.Fatalf("StallClassOf(StallDiamondMix) = %d, want StallUnmodelled", got)
	}
	if got := StallName(StallDiamondMix); got != "diamondmix" {
		t.Fatalf("StallName(StallDiamondMix) = %q, want diamondmix", got)
	}
}

// CheckDiamondMix asserts a built table's Diamond carries gfDiscHand and,
// when a partner is given, that the partner carries another gap kind and the pair
// is refused as diamondmix naming both cards; with none the table is
// not refused at all (the Diamond-alone board dhBoard models).
func CheckDiamondMix(t *testing.T, tbl *kop.Table, diamond kop.CardID, partners ...kop.CardID) {
	t.Helper()
	tt := new(kop.Table)
	*tt = *tbl
	p := prepare(tt)
	if p.gen == nil || !p.gen.has(CardID(diamond), gfDiscHand) || !p.gen.anyDiscHand {
		t.Fatalf("precondition: %s carries no gfDiscHand", tbl.Cards[diamond].Name)
	}
	if dhMixGap(tt, p.gen, p, int(diamond)) {
		t.Fatalf("the Diamond %s counts as a gap of another kind itself", tbl.Cards[diamond].Name)
	}
	r := pairRefusalN(tbl, 2)
	if len(partners) == 0 {
		if p.gen.dhMix {
			t.Fatal("a Diamond-alone table set dhMix")
		}
		if r != nil {
			t.Fatalf("a Diamond-alone table refused: %s: %s", r.Reason, r.Why)
		}
		return
	}
	partner := partners[0]
	pn := tbl.Cards[partner].Name
	if !dhMixGap(tt, p.gen, p, int(partner)) {
		t.Fatalf("precondition: %s carries no census-gap kind (flags %#x, anyScriptGap %v)", pn, p.gen.flags[partner], p.gen.anyScriptGap)
	}
	if r == nil {
		t.Fatalf("Diamond + %s: not refused", pn)
	}
	if r.Reason != "diamondmix" {
		t.Fatalf("Diamond + %s: refused as %s (%s), want diamondmix", pn, r.Reason, r.Why)
	}
	if !slices.Contains(r.Cards, diamond) || !slices.Contains(r.Cards, partner) {
		t.Fatalf("Diamond + %s: refusal names %v, want both %d and %d", pn, r.Cards, diamond, partner)
	}
}
