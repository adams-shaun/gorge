package deck

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// partnerTextFixture is a registry of legendary creatures carrying each
// partner ability shape (CR 702.124h/i): plain partner, two partner—[text]
// variants, and a card with both partner and partner—Survivors (CR
// 702.124g). The scripts are authored here, not copied from Forge.
func partnerTextFixture(t *testing.T) *cards.Registry {
	t.Helper()
	r := cards.NewRegistry()
	for name, kw := range map[string]string{
		"Plain One":   "K:Partner\n",
		"Plain Two":   "K:Partner\n",
		"Select One":  "K:Partner:Character select\n",
		"Select Two":  "K:Partner:Character select\n",
		"Friends One": "K:Partner:Friends forever\n",
		"Both Kinds":  "K:Partner\nK:Partner:Survivors\n",
		"Survivor":    "K:Partner:Survivors\n",
	} {
		src := "Name:" + name + "\nManaCost:W\nTypes:Legendary Creature Human\nPT:2/2\n" + kw
		c, diags := cards.ParseBytes("fixture.txt", []byte(src))
		if len(diags) > 0 {
			t.Fatalf("fixture %s parse: %v", name, diags)
		}
		r.Add(c)
	}
	c, diags := cards.ParseBytes("fixture.txt", []byte("Name:Plains\nTypes:Basic Land Plains\n"))
	if len(diags) > 0 {
		t.Fatalf("fixture Plains parse: %v", diags)
	}
	r.Add(c)
	return r
}

// TestIsPartnerPairPartnerTextMustMatch pins CR 702.124f/i: partner pairs only
// with partner, and a partner—[text] ability only with the same text. Before
// the fix the pair check read the keyword head alone, so K:Partner and
// K:Partner:Character select (both head "Partner") made a legal pair.
func TestIsPartnerPairPartnerTextMustMatch(t *testing.T) {
	r := partnerTextFixture(t)
	get := func(n string) *cards.Card {
		c, ok := r.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the fixture", n)
		}
		return c
	}
	// the precondition: every half really carries a Partner-headed keyword,
	// so a refusal below is the text comparison and not a missing ability
	for _, n := range []string{"Plain One", "Select One", "Friends One", "Both Kinds", "Survivor"} {
		if partnerHead(get(n)) != "Partner" {
			t.Fatalf("%s: partner head %q, want Partner", n, partnerHead(get(n)))
		}
	}
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"Plain One", "Plain Two", true},     // CR 702.124h
		{"Select One", "Select Two", true},   // CR 702.124i, the same text
		{"Plain One", "Select One", false},   // CR 702.124f
		{"Select One", "Friends One", false}, // two different texts
		{"Both Kinds", "Plain One", true},    // CR 702.124g: it may use partner
		{"Both Kinds", "Survivor", true},     // or partner—Survivors
		{"Both Kinds", "Select One", false},
	} {
		a, b := get(tc.a), get(tc.b)
		if got := IsPartnerPair(a, b); got != tc.want {
			t.Errorf("IsPartnerPair(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
		if got := IsPartnerPair(b, a); got != tc.want {
			t.Errorf("IsPartnerPair(%s, %s) = %v, want %v", tc.b, tc.a, got, tc.want)
		}
		if got := IsCommanderPair(a, b); got != tc.want {
			t.Errorf("IsCommanderPair(%s, %s) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
	// the deck validator rejects the mixed pair through the same predicate:
	// the 100-card mono-white deck is legal but for the pair, as the
	// same-text pair's acceptance shows
	deckOf := func(a, b string) File {
		return File{Name: "PAIR", Commanders: []string{a, b}, Cards: []Entry{{a, 1}, {b, 1}, {"Plains", 98}}}
	}
	if f := deckOf("Select One", "Select Two"); f.ValidateCommander(r) != nil {
		t.Fatalf("ValidateCommander(two partner—Character select) = %v, want it legal", f.ValidateCommander(r))
	}
	if f := deckOf("Plain One", "Select One"); f.ValidateCommander(r) == nil || !strings.Contains(f.ValidateCommander(r).Error(), "not a legal commander pair") {
		t.Fatalf("ValidateCommander(partner + partner—Character select) = %v, want a not-a-legal-pair rejection", f.ValidateCommander(r))
	}
}
