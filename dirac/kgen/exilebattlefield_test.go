package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// exileBfSynth lowers a synthetic card script and returns its refusals.
func exileBfSynth(t *testing.T, src string) []refusal {
	t.Helper()
	card, diags := cards.ParseBytes("exilebf_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Exile Bf Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false).Refusals
}

// TestLowerCostExileBattlefield: Exile<1/OriginalHost> (The Dominion
// Bracelet's granted ability) lowers to the CostHostExile table byte;
// Exile<N/Spec> from the battlefield (Fear of Abduction's additional cost, The
// Soul Stone's activated cost) no longer refuses on the cost and the cards name
// their next refusal; each out-of-shape variant still refuses by name.
func TestLowerCostExileBattlefield(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lowerNamed := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}

	// The Dominion Bracelet: the cost lowers; the card stays refused at its
	// granted ability's api (ControlPlayer), which the cost refusal hid.
	lc := lowerNamed("The Dominion Bracelet")
	if got := keclReason(lc.Refusals); got != "AB$ ControlPlayer: api ControlPlayer" {
		t.Errorf("The Dominion Bracelet: refusals %q, want only the ControlPlayer api", got)
	}

	// Fear of Abduction lowers whole; The Soul Stone names its next blocker.
	if got := keclReason(lowerNamed("Fear of Abduction").Refusals); got != "" {
		t.Errorf("Fear of Abduction: refusals %q, want none", got)
	}
	got := keclReason(lowerNamed("The Soul Stone").Refusals)
	if strings.Contains(got, "cost token") || strings.Contains(got, "exile cost") {
		t.Errorf("The Soul Stone still refuses on its exile cost: %q", got)
	}
	if !strings.Contains(got, `unknown property "harnessed"`) {
		t.Errorf("The Soul Stone: refusals %q, want the harnessed property", got)
	}

	// A synthetic Equipment whose granted ability exiles its host (the
	// Bracelet's cost shape): lowers to the CostHostExile table byte.
	const equip = "Name:Exile Bf Probe\nManaCost:2\nTypes:Artifact Equipment\n" +
		"S:Mode$ Continuous | AffectedDefined$ Equipped | Affected$ Creature | AddAbility$ ABProbe | Description$ probe\n" +
		"SVar:ABProbe:AB$ GainLife | Cost$ 1 Exile<1/OriginalHost/Exile Bf Probe> | LifeAmount$ 1 | SpellDescription$ probe\n" +
		"K:Equip:1\n"
	card, diags := cards.ParseBytes("exilebf_equip.txt", []byte(equip))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	ep := &pool{names: []string{"Exile Bf Probe"}, byName: map[string]*cards.Card{"Exile Bf Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	elc := lowerCard(ep, "Exile Bf Probe", card, false)
	if got := keclReason(elc.Refusals); got != "" {
		t.Fatalf("host-exile Equipment refused: %q", got)
	}
	_, eall, _, err := buildPool(&lowered{cards: []*lCard{elc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	saw := false
	for _, c := range eall {
		for i := range c.def.Abilities {
			if cost := &c.def.Abilities[i].Cost; cost.Host == kop.CostHostExile {
				saw = true
				if cost.Flags&(kop.CostSacSelf|kop.CostSac) != 0 || cost.ExileBf {
					t.Errorf("host exile cost flags %b ExileBf %v, want neither a sacrifice nor an exile pick", cost.Flags, cost.ExileBf)
				}
			}
		}
	}
	if !saw {
		t.Error("the host-exile Equipment built with no CostHostExile ability")
	}

	const act = "Name:Exile Bf Probe\nManaCost:1\nTypes:Artifact\nA:AB$ GainLife | Cost$ %s | LifeAmount$ 1\n"
	if rs := exileBfSynth(t, strings.Replace(act, "%s", "1 Exile<2/Creature>", 1)); len(rs) != 0 {
		t.Errorf("Exile<2/Creature> on an activated ability refused: %q", keclReason(rs))
	}
	for _, tc := range []struct{ cost, want string }{
		{"Exile<2/CARDNAME>", `cost token "Exile<2/CARDNAME>"`},            // a count on the source
		{"Exile<1/Creature> Sac<1/Artifact>", "beside a second sacrifice"}, // two sacrifice-class parts
		{"Sac<1/Artifact> Exile<1/Creature>", "beside a sacrifice part"},   // two sacrifice-class parts, other order
		{"Exile<1/CARDNAME> Exile<1/Creature>", `cost token`},              // exile-self beside an exile pick
	} {
		got := keclReason(exileBfSynth(t, strings.Replace(act, "%s", tc.cost, 1)))
		if !strings.Contains(got, tc.want) {
			t.Errorf("Cost$ %s: refusals %q, want %q", tc.cost, got, tc.want)
		}
	}
}
