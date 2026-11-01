package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func kcastgraveSynthetic(kind, api string, params map[string]string) *lCard {
	face := &cards.Face{
		Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"},
		Abilities: []*cards.SA{{Kind: kind, API: api, Params: params}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

// TestCastgraveConditionArms exercises the EQ0 negation arm (no carrier) and
// a permanent's own condition, which reads false in the kernel but true in
// gorge and therefore refuses.
func TestCastgraveConditionArms(t *testing.T) {
	neg := kcastgraveSynthetic("SP", "Draw", map[string]string{
		"Defined": "You", "NumCards": "1",
		"ConditionDefined": "Self", "ConditionPresent": "Card.wasCastFromGraveyard", "ConditionCompare": "EQ0",
	})
	if len(neg.Refusals) != 0 {
		t.Fatalf("EQ0 arm refused: %v", neg.Refusals)
	}
	if len(neg.Abilities) != 1 {
		t.Fatalf("EQ0 arm lowered to %v", neg.Abilities)
	}
	var c lCond
	for _, in := range neg.Abilities[0].Prog {
		if in.Cond.Kind != cNone {
			c = in.Cond
			break
		}
	}
	if c.Kind != cValue || c.Op != opEQ || c.B.N != 0 || c.A.Kind != vCastFromGrave || len(c.A.Sub) != 2 ||
		c.A.Sub[0].N != 1 || c.A.Sub[1].N != 0 {
		t.Errorf("EQ0 arm lowered to %+v, want Value(EQ0, CastFromGrave(1,0))", c)
	}

	// A permanent's own activated ability reads the resolving stack entry
	// (an ability), not its cast spell, so the condition refuses.
	perm := kcastgraveSynthetic("AB", "Draw", map[string]string{
		"Cost": "1 B", "Defined": "You", "NumCards": "1",
		"ConditionDefined": "Self", "ConditionPresent": "Card.wasCastFromGraveyard",
	})
	if !hasRefusal(perm.Refusals, "outside a spell ability") {
		t.Errorf("permanent condition not refused: %v", perm.Refusals)
	}

	// The count head has the same restriction.
	count := kcastgraveSynthetic("AB", "Draw", map[string]string{
		"Cost": "1 B", "Defined": "You", "NumCards": "Count$wasCastFromGraveyard.1.0",
	})
	if !hasRefusal(count.Refusals, "outside a spell ability") {
		t.Errorf("permanent count head not refused: %v", count.Refusals)
	}
}

func hasRefusal(refs []refusal, want string) bool {
	for _, r := range refs {
		if strings.Contains(r.Reason, want) {
			return true
		}
	}
	return false
}
