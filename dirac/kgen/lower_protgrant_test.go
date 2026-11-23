package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kprotRefusals lowers an Equipment probe carrying the static and returns
// its refusal reasons joined. The pool also carries an Aura, so its subtype
// word is known to the filter parser (a pool with no Aura cannot read the
// Spectra Ward "Card.Aura" exception).
func kprotRefusals(t *testing.T, static string) string {
	t.Helper()
	src := strings.Join([]string{"Name:Probe", "ManaCost:1", "Types:Artifact Equipment", static}, "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	ward, diags := cards.ParseBytes("Wardprobe.txt", []byte("Name:Wardprobe\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant:Creature"))
	if ward == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := ward.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe", "Wardprobe"}, byName: map[string]*cards.Card{"Probe": card, "Wardprobe": ward},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, r := range lowerPool(p).cards[0].Refusals {
		out = append(out, r.Reason)
	}
	return strings.Join(out, "; ")
}

// TestStaticProtectionGrant: a Continuous static's AddKeyword$ Protection:
// <spec> lowers to the static's ProtFrom quality (no keyword bit is free
// for it), several qualities share one filter, a granted protection lowers
// from a creature host (gap4-05), and a player quality refuses by name.
func TestStaticProtectionGrant(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Sword of Wealth and Power", form: "static AddKeyword$ Protection:Instant & Protection:Sorcery", lir: "protfrom="},
	})
	if got := kcantselfRefusals(t, "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Creature | Description$ x"); got != "" {
		t.Errorf("a creature host grant: want it lowered, got %q", got)
	}
	for _, c := range []struct{ name, static, want string }{
		{"a type quality lowers", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Instant | Description$ x", ""},
		{"a type quality beside another keyword lowers", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Sorcery & Flying | Description$ x", ""},
		{"a subtype quality lowers", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Goblin | Description$ x", ""},
		{"a coloured quality with an exception lowers", "S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddKeyword$ Protection:Card.White:white:Card.CardUID_HostCardUID | Description$ x", ""},
		{"an Aura quality with an exception lowers", "S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddKeyword$ Protection:Card.nonColorless:each color:Aura | Description$ x", ""},
		{"protection from opponents refuses", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Protection:Player.Opponent:each of your opponents | Description$ x", `keyword "Protection:Player.Opponent:each of your opponents"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := kprotRefusals(t, c.static)
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
