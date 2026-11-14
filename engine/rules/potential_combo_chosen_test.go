package rules

import "testing"

// comboChosenColours reads a gate land's "Combo U Chosen" production: the
// colours listed beside the chosen one, and only that shape.
func TestComboChosenColours(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string
		ok   bool
	}{
		{"Combo U Chosen", "U", true},
		{"Combo W Chosen", "W", true},
		{"Combo Chosen", "", true},
		{"Combo W U Chosen", "W U", true},
		{"Chosen", "", false},
		{"Combo Any", "", false},
		{"Combo W U", "", false},
		{"Combo UU Chosen", "", false},
		{"Combo X Chosen", "", false},
		{"Combo ColorIdentity", "", false},
		{"", "", false},
	} {
		got, ok := comboChosenColours(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("comboChosenColours(%q) = %q, %v; want %q, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}
