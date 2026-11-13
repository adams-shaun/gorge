package deck

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestIsDraftCommanderPair pins the Commander-draft partner rule: a
// commander-eligible card of at most one colour with no partner-family
// ability may be treated as having partner, so it pairs with another such
// card or with a plain-Partner card, but never with a Partner-with, Doctor's
// companion or two-colour commander, a non-commander, or another copy of
// itself.
func TestIsDraftCommanderPair(t *testing.T) {
	r := commanderFixture(t)
	get := func(name string) *cards.Card {
		c, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("fixture %q missing", name)
		}
		return c
	}
	cases := []struct {
		a, b string
		want bool
	}{
		{"Amalia", "Ember Dragon", true},            // two mono-colour commanders
		{"Amalia", "Golos, Tireless Pilgrim", true}, // mono-colour with colourless
		{"Amalia", "Plain Partner", true},           // implied partner with printed partner
		{"Hobbit Warden", "Garden Keeper", true},    // an ordinary legal pair stays legal
		{"Amalia", "Hobbit Warden", false},          // Partner with names someone else
		{"Amalia", "Gallifrey Envoy", false},        // Doctor's companion is its own family
		{"Amalia", "Genesis Engine", false},         // a two-colour commander has no implied partner
		{"Amalia", "Knight", false},                 // not a commander
		{"Amalia", "Amalia", false},                 // two copies of one card
	}
	for _, c := range cases {
		if got := IsDraftCommanderPair(get(c.a), get(c.b)); got != c.want {
			t.Errorf("IsDraftCommanderPair(%s, %s) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
	if IsCommanderPair(get("Amalia"), get("Ember Dragon")) {
		t.Error("IsCommanderPair accepts two partnerless commanders; the draft rule must stay opt-in")
	}
	if HasImpliedDraftPartner(get("Plain Partner")) {
		t.Error("a card that already has partner is not treated as gaining it")
	}
}
