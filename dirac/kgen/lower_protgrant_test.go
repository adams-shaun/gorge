package kgen

import (
	"strings"
	"testing"
)

// TestStaticProtectionGrant: a Continuous static's AddKeyword$ Protection:
// <spec> lowers to the static's ProtFrom quality (no keyword bit is free
// for it), several qualities share one filter, and a subtype or a player
// quality refuses by name.
func TestStaticProtectionGrant(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Sword of Wealth and Power", form: "static AddKeyword$ Protection:Instant & Protection:Sorcery", lir: "protfrom="},
	})
	for _, c := range []struct{ name, static, want string }{
		{"a type quality lowers", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Instant | Description$ x", ""},
		{"a type quality beside another keyword lowers", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Sorcery & Flying | Description$ x", ""},
		{"a subtype quality refuses", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Goblin | Description$ x", "names a subtype"},
		{"protection from opponents refuses", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Player.Opponent:each of your opponents | Description$ x", `keyword "Protection:Player.Opponent:each of your opponents"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kcantselfRefusals(t, c.static)
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
