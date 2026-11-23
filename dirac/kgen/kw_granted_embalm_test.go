package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExpandedKeywordsGrantedEmbalmLower: Cursecloth Wrappings' pump grants
// Embalm:CardManaCost to a creature card in its owner's graveyard. It lowers
// whole: the pump's body is OpGrantEmbalmOwnCost on target 0 (the graveyard
// card), naming the card's latent embalm body, which is gorge's own kwEmbalm
// expansion (a graveyard, sorcery-speed CopyAsToken of Self, exiling the card
// as its cost). Every other granted keyword stays refused by name, the
// granted Eternalize included.
func TestExpandedKeywordsGrantedEmbalmLower(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Cursecloth Wrappings")
	if !ok || card == nil {
		t.Fatal("Cursecloth Wrappings not in the corpus")
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	if len(lc.Refusals) != 0 {
		t.Fatalf("Cursecloth Wrappings refusals = %v, want none", lc.Refusals)
	}
	var pump *lAbility
	for _, ab := range lc.Abilities {
		if len(ab.Targets) == 1 {
			pump = ab
		}
	}
	if pump == nil {
		t.Fatalf("no targeting pump ability among %d abilities", len(lc.Abilities))
	}
	if tg := pump.Targets[0]; tg.Zone != zGraveyard {
		t.Fatalf("the pump's target zone = %d, want the graveyard", tg.Zone)
	}
	if len(pump.Prog) != 1 {
		t.Fatalf("pump program = %v, want the one GrantEmbalmOwnCost", pump.Prog)
	}
	in := pump.Prog[0]
	if in.Op != "GrantEmbalmOwnCost" || in.Obj.Kind != rTarget || in.Obj.N != 0 {
		t.Fatalf("pump instruction %+v, want GrantEmbalmOwnCost on target 0", in)
	}
	if in.C < 0 || in.C >= len(lc.Abilities) {
		t.Fatalf("GrantEmbalmOwnCost names ability %d of %d", in.C, len(lc.Abilities))
	}
	body := lc.Abilities[in.C]
	if !body.Latent {
		t.Errorf("the embalm body is not latent: the card would offer it itself")
	}
	if body.Zone != zGraveyard || !body.Sorcery || body.Cost.ExileG == nil {
		t.Errorf("the embalm body is not a sorcery-speed graveyard self-exile ability: zone %d cost %s", body.Zone, costString(body.Cost))
	}
	if len(body.Prog) != 1 || body.Prog[0].Op != "CopyAsToken" || body.Prog[0].Obj.Kind != rSelf {
		t.Errorf("the embalm body program = %v, want one CopyAsToken of Self", body.Prog)
	}

	// the class stays refused by name for every other granted keyword
	for _, kw := range []string{"Eternalize:CardManaCost", "Embalm:2 W", "Soulshift:3"} {
		if _, err := pumpKeywords(kw); err == nil {
			t.Errorf("pumpKeywords(%q) lowers: want the refusal", kw)
		}
	}
}
