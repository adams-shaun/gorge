package rules

import "testing"

// Two SetMaxHandSize$ statics affecting one player apply in timestamp order
// (CR 613.7; Reliquary Tower's ruling), so the later one sets the maximum: a
// "no maximum" static then a "maximum is ten" static leaves ten, and the
// reverse order leaves no maximum.

const (
	noMaxHandSrc = "Name:Test Tower\nManaCost:2\nTypes:Artifact\n" +
		"S:Mode$ Continuous | SetMaxHandSize$ Unlimited | Affected$ You | Description$ test: unlimited hand.\nOracle:x\n"
	tenMaxHandSrc = "Name:Test Rings\nManaCost:8\nTypes:Artifact\n" +
		"S:Mode$ Continuous | SetMaxHandSize$ 10 | Affected$ You | Description$ test: hand of ten.\nOracle:x\n"
)

// TestMaxHandSizeLatestTimestampWins puts the two statics on the battlefield
// in each order and asserts the later one's value. The precondition checks
// that the second permanent's layer timestamp is the later one, so the
// assertion reads the order the test claims.
func TestMaxHandSizeLatestTimestampWins(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		first, second string
		want          int
	}{
		{"unlimited then ten", noMaxHandSrc, tenMaxHandSrc, 10},
		{"ten then unlimited", tenMaxHandSrc, noMaxHandSrc, unlimitedHandSize},
	} {
		e := mayPlayBase(t)
		a := onBoardGrant(t, e, 0, tc.first)
		b := onBoardGrant(t, e, 0, tc.second)
		if ta, tb := e.G.Obj(a).LayerTimestamp(), e.G.Obj(b).LayerTimestamp(); tb <= ta {
			t.Fatalf("%s: precondition: second timestamp %d not after first %d", tc.name, tb, ta)
		}
		if got := e.maxHandSizeFor(0); got != tc.want {
			t.Errorf("%s: max hand size = %d, want %d (the later static, CR 613.7)", tc.name, got, tc.want)
		}
		if got := e.maxHandSizeFor(1); got != maxHandSize {
			t.Errorf("%s: opponent max hand size = %d, want the default %d", tc.name, got, maxHandSize)
		}
	}
}
