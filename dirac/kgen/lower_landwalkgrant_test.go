package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// landwalkGrantProbe lowers a Goblin creature probe carrying the static and
// returns its refusal reasons joined, and its lowered-IR dump.
func landwalkGrantProbe(t *testing.T, static string) (refusals, lir string) {
	t.Helper()
	src := strings.Join([]string{"Name:Probe", "ManaCost:1 R R", "Types:Creature Goblin", "PT:2/2", static}, "\n")
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	var out []string
	for _, r := range lw.cards[0].Refusals {
		out = append(out, r.Reason)
	}
	var b strings.Builder
	dumpLIR(&b, lw.cards[0])
	return strings.Join(out, "; "), b.String()
}

// TestStaticLandwalkGrant: a Continuous static's AddKeyword$ Landwalk:<spec>
// lowers to the static's Landwalk land filter (no keyword bit is free for it;
// Goblin King, Lord of Atlantis), and a grant from an attached host or with no
// land spec refuses by name.
func TestStaticLandwalkGrant(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Goblin King", form: "static AddKeyword$ Landwalk:Mountain", lir: "landwalk="},
		{card: "Lord of Atlantis", form: "static AddKeyword$ Landwalk:Island", lir: "landwalk="},
	})
	for _, c := range []struct{ name, static, want string }{
		{"an Aura's grant refuses", "S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddKeyword$ Landwalk:Forest | Description$ x", "attached host"},
		{"an Equipment's grant refuses", "S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddKeyword$ Landwalk:Forest | Description$ x", "attached host"},
		{"a landwalk with no spec refuses", "S:Mode$ Continuous | Affected$ Creature.Other+YouCtrl | AddKeyword$ Landwalk: | Description$ x", "no land spec"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got, _ := landwalkGrantProbe(t, c.static); !strings.Contains(got, c.want) {
				t.Errorf("want refusal %q, got %q", c.want, got)
			}
		})
	}
}
