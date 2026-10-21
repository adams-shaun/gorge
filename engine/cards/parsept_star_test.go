package cards

import "testing"

// TestParsePTReadsTheStarAsZero: CR 208.2a -- a printed star is the
// characteristic-defining ability's value, so with no CDA applying it is 0
// and a side like "1+*" is its fixed term. Every shape the corpus prints.
func TestParsePTReadsTheStarAsZero(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		pt       string
		pow, tgh int32
		cd       bool
	}{
		{"2/2", 2, 2, false},
		{"*/*", 0, 0, true},
		{"*/1+*", 0, 1, true},
		{"1+*/1+*", 1, 1, true},
		{"2+*/2+*", 2, 2, true},
		{"*/*+1", 0, 1, true},
		{"*/7-*", 0, 7, true},
		{"1+*/3", 1, 3, true},
		{"0/*", 0, 0, true},
		{"", 0, 0, false},
	} {
		if p, tg, cd := parsePT(c.pt); p != c.pow || tg != c.tgh || cd != c.cd {
			t.Errorf("CR 208.2a parsePT(%q) = %d/%d cd=%v, want %d/%d cd=%v", c.pt, p, tg, cd, c.pow, c.tgh, c.cd)
		}
	}
}
