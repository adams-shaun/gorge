package kgen

import "testing"

// evolveGrantForms: a static's AddKeyword$ Evolve lowers as a latent ETB
// trigger of the printed expansion's shape (Tf2Evolve), listed once per grant
// (CR 702.100a, 113.2c).
var evolveGrantForms = []formCase{
	{card: "Tyranid Prime", form: "static AddKeyword$ Evolve: a latent GrantedEvolve trigger beside the printed one", lir: "GrantedEvolve"},
	{card: "Propagator Drone", form: "static AddKeyword$ Evolve on tokens: a latent GrantedEvolve trigger", lir: "GrantedEvolve"},
}

func TestEvolveGrantLowers(t *testing.T) { checkForms(t, evolveGrantForms) }

// TestEvolveGrantSplit: every bare Evolve term is its own instance; any other
// keyword stays in the list.
func TestEvolveGrantSplit(t *testing.T) {
	for _, c := range []struct {
		kw, rest string
		n        int
	}{
		{"Evolve", "", 1},
		{"Flying & Evolve", "Flying", 1},
		{"Evolve & Evolve", "", 2},
		{"Flying", "Flying", 0},
		{"Evolve:1", "Evolve:1", 0},
	} {
		rest, n := splitEvolveGrant(c.kw)
		if rest != c.rest || n != c.n {
			t.Errorf("splitEvolveGrant(%q) = %q, %d; want %q, %d", c.kw, rest, n, c.rest, c.n)
		}
	}
}
