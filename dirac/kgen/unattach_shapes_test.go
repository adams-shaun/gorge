package kgen

import (
	"strings"
	"testing"
)

// unattachForms covers api:Unattach. A Defined$ that names one object
// (Enchanted, ChosenCard) lowers to the Unattach extension; a Defined$ that
// names a group refuses by name.
var unattachForms = []formCase{
	{card: "Carry Away", form: "Defined$ Enchanted", lir: "Unattach"},
}

func TestUnattachShapes(t *testing.T) { checkForms(t, unattachForms) }

// TestUnattachGroupDefinedRefuses: a Defined$ that names a group, and a missing
// one, refuse by name (gorge walks the whole Defined set; the kernel's detach
// reads one object).
func TestUnattachGroupDefinedRefuses(t *testing.T) {
	const base = "Name:Unattach Probe\nManaCost:1 W\nTypes:Sorcery\nA:SP$ Unattach"
	cases := []struct{ name, extra, want string }{
		{"one object", " | Defined$ Enchanted\n", ""},
		{"remembered group", " | Defined$ Remembered\n", "Unattach Defined$ Remembered"},
		{"no defined", "\n", "Unattach with no Defined$"},
	}
	for _, c := range cases {
		got := keclReason(keclSynth(t, "Unattach Probe", base+c.extra))
		if c.want == "" {
			if got != "" {
				t.Errorf("%s: refused: %s", c.name, got)
			}
		} else if !strings.Contains(got, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, got, c.want)
		}
	}
}
