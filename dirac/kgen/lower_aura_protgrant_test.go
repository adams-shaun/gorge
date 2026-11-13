package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// auraProtRefusals lowers an Aura probe carrying the static and returns its
// refusal reasons joined.
func auraProtRefusals(t *testing.T, static string) string {
	t.Helper()
	src := strings.Join([]string{"Name:Probe", "ManaCost:1 W", "Types:Enchantment Aura", "K:Enchant:Creature", static}, "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, r := range lowerPool(p).cards[0].Refusals {
		out = append(out, r.Reason)
	}
	return strings.Join(out, "; ")
}

// TestAuraProtectionGrant: an Aura's Continuous AddKeyword$ Protection:
// <spec> on the enchanted creature lowers like an Equipment's (Spirit Mantle,
// Unquestioned Authority), while a chosen-colour quality and one with an
// exception field (Pentarch Ward, Benevolent Blessing) stay refused: gorge
// derives those keywords without the host's choice or exception.
func TestAuraProtectionGrant(t *testing.T) {
	for _, c := range []struct{ name, static, want string }{
		{"protection from creatures lowers", "S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddPower$ 1 | AddToughness$ 1 | AddKeyword$ Protection:Creature | Description$ x", ""},
		{"a chosen-colour quality refuses", "S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddKeyword$ Protection:Card.ChosenColor:chosenColor | Description$ x", "ChosenColor"},
		{"an exception field refuses", "S:Mode$ Continuous | AffectedDefined$ Enchanted | Affected$ Creature | AddKeyword$ Protection:Creature:creatures:Card.CardUID_HostCardUID | Description$ x", "exception field"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := auraProtRefusals(t, c.static)
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
