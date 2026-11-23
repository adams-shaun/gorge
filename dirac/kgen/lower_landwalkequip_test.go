package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// landwalkEquipProbe lowers an Equipment probe carrying the static and
// returns its refusal reasons joined and its lowered-IR dump.
func landwalkEquipProbe(t *testing.T, static string) (refusals, lir string) {
	t.Helper()
	src := strings.Join([]string{"Name:Probe", "ManaCost:2", "Types:Artifact Equipment", "K:Equip:2", static}, "\n")
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

// TestEquipmentLandwalkGrant: an Equipment's AddKeyword$ Landwalk:<spec>
// (Trailblazer's Boots) lowers to the static's land filter beside its
// equipped-creature filter, and an Aura's grant (Burrowing) lowers the same way.
func TestEquipmentLandwalkGrant(t *testing.T) {
	got, lir := landwalkEquipProbe(t, "S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddKeyword$ Landwalk:Land.nonBasic:Nonbasic land | Description$ x")
	if got != "" || !strings.Contains(lir, "landwalk=") {
		t.Errorf("an Equipment's landwalk grant: refusals %q, lir %q", got, lir)
	}
	if got, lir := landwalkGrantProbe(t, "S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddKeyword$ Landwalk:Land.nonBasic:Nonbasic land | Description$ x"); got != "" || !strings.Contains(lir, "landwalk=") {
		t.Errorf("an Aura's landwalk grant: refusals %q, lir %q, want it lowered", got, lir)
	}
}

// TestPumpLandwalkLowers: a Pump's KW$ Landwalk:<spec> (Piracy Charm's
// islandwalk until end of turn) is an end-of-turn override record carrying the
// land filter, and Trailblazer's Boots lowers whole.
func TestPumpLandwalkLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Piracy Charm", "Trailblazer's Boots"} {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
		lw := lowerPool(p)
		if r := lw.cards[0].Refusals; len(r) != 0 {
			t.Fatalf("%s refusals = %v, want none", name, r)
		}
		var b strings.Builder
		dumpLIR(&b, lw.cards[0])
		want := "landwalk="
		if !strings.Contains(b.String(), want) {
			t.Errorf("%s lowered IR has no %q:\n%s", name, want, b.String())
		}
	}
}
