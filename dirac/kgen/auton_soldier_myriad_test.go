package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Auton Soldier's "enter as a copy of any creature, except ... it has
// myriad" lowers: the BecomeCopy grant override names the latent
// GrantedMyriad trigger (gorge folds the keyword into the copy face; the
// kernel grants the same trigger through the as-enters grant record). A
// CopyPermanent exception (Muddle, the Ever-Changing) still refuses.
func TestAutonSoldierCopyMyriadLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	lower := func(name string) *lCard {
		card, ok := reg.Lookup(name)
		if !ok || card == nil {
			t.Fatalf("%s not in the corpus", name)
		}
		p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
		return lowerCard(p, card.Faces[0].Name, card, false)
	}

	as := lower("Auton Soldier")
	if len(as.Refusals) != 0 {
		t.Fatalf("Auton Soldier refusals = %v, want none", as.Refusals)
	}
	granted := 0
	for _, o := range as.overrides {
		for _, idx := range o.GrantAbility {
			if a := as.Abilities[idx]; a.Latent && a.Name == "GrantedMyriad" {
				granted++
			}
		}
	}
	if granted != 1 {
		t.Fatalf("grant overrides name GrantedMyriad %d times, want 1 (overrides %d)", granted, len(as.overrides))
	}

	mu := lower("Muddle, the Ever-Changing")
	if len(mu.Refusals) == 0 {
		t.Fatal("Muddle, the Ever-Changing lowers: a CopyPermanent Myriad exception must still refuse")
	}
	found := false
	for _, r := range mu.Refusals {
		if strings.Contains(r.Reason, "copy exception AddKeywords$ Myriad: ") {
			found = true
		}
	}
	if !found {
		t.Fatalf("Muddle refusals = %v, want the Myriad copy-exception reason", mu.Refusals)
	}
}
