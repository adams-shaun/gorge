package kgen

import "testing"

// exaltedGrantForms: a static's AddKeyword$ Exalted lowers as a latent
// Attacks trigger of the printed expansion's shape, listed once per grant
// (CR 702.83b); the card's own printed Exalted is its first Attacks trigger.
var exaltedGrantForms = []formCase{
	{card: "Sublime Archangel", form: "static AddKeyword$ Exalted: a latent GrantedExalted trigger (Tf2Alone) beside the printed one", lir: "GrantedExalted"},
}

func TestGrantedExaltedLowers(t *testing.T) { checkForms(t, exaltedGrantForms) }

// TestExaltedGrantSplit: every bare Exalted term is its own instance; any
// other keyword stays in the list.
func TestExaltedGrantSplit(t *testing.T) {
	for _, c := range []struct {
		kw, rest string
		n        int
	}{
		{"Exalted", "", 1},
		{"Flying & Exalted", "Flying", 1},
		{"Exalted & Exalted", "", 2},
		{"Flying", "Flying", 0},
		{"Exalted:1", "Exalted:1", 0},
	} {
		rest, n := splitExaltedGrant(c.kw)
		if rest != c.rest || n != c.n {
			t.Errorf("splitExaltedGrant(%q) = %q, %d; want %q, %d", c.kw, rest, n, c.rest, c.n)
		}
	}
}
