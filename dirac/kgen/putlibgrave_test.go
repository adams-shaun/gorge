package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kputLibSynthetic lowers one synthetic ability carrying a PutCardToLib cost.
func kputLibSynthetic(kind, api string, params map[string]string) *lCard {
	face := &cards.Face{
		Name: "Synthetic", ManaCost: "2 B G", Types: []string{"Creature"},
		Abilities: []*cards.SA{{Kind: kind, API: api, Params: params}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

// TestPutCardToLibFromGraveSelfCost pins Daniels, Temporal Agent's
// ActivateZone$ Graveyard ability: PutCardToLibFromGrave<1/-1/CARDNAME/this
// card> is the source itself to the bottom of its owner's library as the
// cost (CR 118.3, 602.2b), lowered to Cost.PutLibSelf beside the {2}; the
// enters trigger returning a creature card stays untouched.
func TestPutCardToLibFromGraveSelfCost(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Daniels, Temporal Agent"})
	var c *lCard
	for _, x := range lw.cards {
		if x.Name == "Daniels, Temporal Agent" {
			c = x
		}
	}
	if c == nil {
		t.Fatal("Daniels, Temporal Agent not lowered")
	}
	for _, r := range c.Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	var ab *lAbility
	for _, a := range c.Abilities {
		if a.Kind == akActivated {
			ab = a
		}
	}
	if ab == nil {
		t.Fatal("no activated ability")
	}
	if !ab.Cost.PutLibSelf {
		t.Error("the activated cost does not carry PutLibSelf")
	}
	if ab.Zone != zGraveyard {
		t.Errorf("activation zone = %d, want graveyard", ab.Zone)
	}
	if ab.Cost.Mana.Generic != 2 || ab.Cost.Mana.Pips != [5]int{} {
		t.Errorf("mana cost = %v, want {2}", ab.Cost.Mana)
	}
	var trig *lAbility
	for _, a := range c.Abilities {
		if a.Kind == akTriggered {
			trig = a
		}
	}
	if trig == nil || len(trig.Prog) == 0 || trig.Prog[0].Op != "Return" || len(trig.Targets) != 1 ||
		trig.Targets[0].Zone != zGraveyard {
		t.Fatalf("the enters trigger's return-from-graveyard body changed: %+v", trig)
	}
}

// TestPutCardToLibFromGraveRefusals keeps every other shape refused: another
// position or spec, a count other than one, the Hand/Battlefield heads, a
// battlefield activation and a spell cost.
func TestPutCardToLibFromGraveRefusals(t *testing.T) {
	graveAB := func(cost string) map[string]string {
		return map[string]string{"Cost": cost, "ActivationZone": "Graveyard", "Defined": "You", "LifeAmount": "2"}
	}
	cases := []struct {
		name   string
		kind   string
		api    string
		params map[string]string
		want   string
	}{
		{"another position", "AB", "GainLife", graveAB("PutCardToLibFromGrave<1/0/CARDNAME>"), "put-card-to-library cost"},
		{"another spec", "AB", "GainLife", graveAB("PutCardToLibFromGrave<1/-1/Card>"), "put-card-to-library cost"},
		{"two cards", "AB", "GainLife", graveAB("PutCardToLibFromGrave<2/-1/CARDNAME>"), "put-card-to-library cost"},
		{"hand head", "AB", "GainLife", graveAB("PutCardToLibFromHand<1/-1/CARDNAME>"), "PutCardToLibFromHand"},
		{"battlefield head", "AB", "GainLife", graveAB("PutCardToLibFromBattlefield<1/-1/CARDNAME>"), "PutCardToLibFromBattlefield"},
		{"battlefield activation", "AB", "GainLife", map[string]string{"Cost": "PutCardToLibFromGrave<1/-1/CARDNAME>", "Defined": "You", "LifeAmount": "2"}, "outside a graveyard activated ability"},
		{"spell cost", "SP", "Draw", map[string]string{"Cost": "PutCardToLibFromGrave<1/-1/CARDNAME>", "Defined": "You", "NumCards": "1"}, "outside a graveyard activated ability"},
	}
	for _, tc := range cases {
		c := kputLibSynthetic(tc.kind, tc.api, tc.params)
		if !hasRefusal(c.Refusals, tc.want) {
			t.Errorf("%s: want a refusal containing %q, got %v", tc.name, tc.want, c.Refusals)
		}
	}
}
