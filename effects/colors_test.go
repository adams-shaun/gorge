package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func objFrom(t *testing.T, src string) *state.Object {
	t.Helper()
	c, diags := cards.ParseBytes("x.txt", []byte(src))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	return &state.Object{ID: 1, Card: c}
}

func TestColorsOf(t *testing.T) {
	cases := []struct{ src, want string }{
		{"Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n", "R"},
		{"Name:Dimir\nManaCost:U B\nTypes:Instant\nOracle:x\n", "UB"},
		{"Name:Colorless\nManaCost:3\nTypes:Artifact\nOracle:x\n", ""},
		{"Name:Colored by line\nManaCost:2\nTypes:Artifact\nColors:green\nOracle:x\n", "G"},
		{"Name:Breaker\nManaCost:6 G\nTypes:Creature Eldrazi\nK:Devoid\nOracle:x\n", ""},
		{"Name:Land\nManaCost:no cost\nTypes:Land\nOracle:x\n", ""},
		{"Name:Hybrid\nManaCost:W/U\nTypes:Instant\nOracle:x\n", "WU"},
	}
	for _, tc := range cases {
		if got := ColorsOf(objFrom(t, tc.src)); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.src[:20], got, tc.want)
		}
	}
	if ColorsOf(&state.Object{}) != "" || ColorsOf(nil) != "" {
		t.Fatal("faceless/nil object is not colourless")
	}
}
