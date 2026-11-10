package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// exileTwoGraveBuilt lowers and builds one synthetic card, returning its
// refusals and the cost of its first ability when it lowered.
func exileTwoGraveBuilt(t *testing.T, src string) ([]refusal, *kop.Cost) {
	t.Helper()
	card, diags := cards.ParseBytes("exiletwograve_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Exile Two Grave Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := lowerCard(p, name, card, false)
	if len(lc.Refusals) != 0 {
		return lc.Refusals, nil
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, c := range all {
		if len(c.def.Abilities) > 0 {
			return nil, &c.def.Abilities[0].Cost
		}
	}
	t.Fatal("the probe built no ability")
	return nil, nil
}

// TestLowerExileTwoGraveParts: ExileFromGrave<1/CARDNAME> followed by
// ExileFromGrave<N/Spec> of the other cards (Say Its Name's "Exile this card
// and two other cards named Say Its Name from your graveyard") lowers to
// CostExileGrave with the Cost.ExileGSelf table byte, no new CostFlags bit;
// every out-of-shape variant still refuses by name.
func TestLowerExileTwoGraveParts(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Say Its Name")
	if !ok || card == nil {
		t.Fatal("Say Its Name is not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	if got := keclReason(lc.Refusals); got != "" {
		t.Fatalf("Say Its Name refused: %q", got)
	}
	_, all, _, err := buildPool(&lowered{cards: []*lCard{lc}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	saw := false
	for _, c := range all {
		for i := range c.def.Abilities {
			cost := &c.def.Abilities[i].Cost
			if cost.Flags&kop.CostExileGrave == 0 {
				continue
			}
			saw = true
			if !cost.ExileGSelf || cost.N != 2 || cost.Filter == 0 {
				t.Errorf("Say Its Name cost: ExileGSelf %v N %d Filter %d, want self, 2 others and a filter", cost.ExileGSelf, cost.N, cost.Filter)
			}
		}
	}
	if !saw {
		t.Fatal("Say Its Name built no exile-from-graveyard cost")
	}

	const act = "Name:Exile Two Grave Probe\nManaCost:1\nTypes:Sorcery\nA:AB$ GainLife | Cost$ %s | ActivationZone$ %z | LifeAmount$ 1\n"
	probe := func(cost, zone string) string {
		return strings.NewReplacer("%s", cost, "%z", zone).Replace(act)
	}
	const two = "ExileFromGrave<1/CARDNAME> ExileFromGrave<2/Card.Other+YouOwn+namedProbe>"
	rs, cost := exileTwoGraveBuilt(t, probe(two, "Graveyard"))
	if len(rs) != 0 {
		t.Fatalf("the synthetic two-part cost refused: %q", keclReason(rs))
	}
	if !cost.ExileGSelf || cost.N != 2 || cost.Flags&kop.CostExileGrave == 0 {
		t.Errorf("synthetic cost: ExileGSelf %v N %d flags %b", cost.ExileGSelf, cost.N, cost.Flags)
	}
	// the one-part forms keep their old lowering: no self byte
	if _, cost := exileTwoGraveBuilt(t, probe("ExileFromGrave<1/CARDNAME>", "Graveyard")); cost == nil || cost.ExileGSelf {
		t.Errorf("the lone CARDNAME exile built ExileGSelf: %+v", cost)
	}

	for _, tc := range []struct{ cost, zone, want string }{
		{"ExileFromGrave<2/Card.Other+YouOwn+namedProbe> ExileFromGrave<1/CARDNAME>", "Graveyard", "second exile cost part"},                                  // the self part second
		{"ExileFromGrave<1/CARDNAME> ExileFromGrave<2/Card.Other+YouOwn+namedProbe> ExileFromGrave<1/Creature.Other>", "Graveyard", "second exile cost part"}, // three parts
		{"ExileFromGrave<1/CARDNAME> ExileFromGrave<2/Card.YouOwn+namedProbe>", "Graveyard", "must exclude the source"},                                       // a spec that admits the source
		{"ExileFromGrave<1/CARDNAME> ExileFromGrave<1/CARDNAME>", "Graveyard", "second exile cost part"},                                                      // the source twice
		{"ExileFromGrave<1/Creature.Other> ExileFromGrave<2/Card.Other+YouOwn+namedProbe>", "Graveyard", "second exile cost part"},                            // a first part that is not the source
		{"ExileFromGrave<1/CARDNAME> ExileCtrlOrGrave<2/Card.Other+YouOwn+namedProbe>", "Graveyard", "second exile cost part"},                                // the battlefield too
		{two, "Battlefield", "outside a graveyard activated ability"},                                                                                         // not a graveyard ability
		{two, "Hand", "outside a graveyard activated ability"},                                                                                                // not a graveyard ability
	} {
		rs, _ := exileTwoGraveBuilt(t, probe(tc.cost, tc.zone))
		if got := keclReason(rs); !strings.Contains(got, tc.want) {
			t.Errorf("Cost$ %s (%s): refusals %q, want %q", tc.cost, tc.zone, got, tc.want)
		}
	}
}
