package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func TestEveryRemembered(t *testing.T) {
	for ct, want := range map[string]bool{
		"Creature.IsRemembered":                   true,
		"Land.IsRemembered,Artifact.IsRemembered": true,
		"Creature":                   false,
		"Creature.IsRemembered,Land": false,
		"Card.!IsRemembered":         false,
		"":                           false,
	} {
		if got := everyRemembered(ct); got != want {
			t.Errorf("everyRemembered(%q) = %v, want %v", ct, got, want)
		}
	}
}

// TestDualOriginChangeAllRefuses: a Graveyard,Exile ChangeZoneAll whose
// ChangeType$ is not restricted to the remembered set must refuse.
func TestDualOriginChangeAllRefuses(t *testing.T) {
	line := "DB$ ChangeZoneAll | Origin$ Graveyard,Exile | Destination$ Hand | ChangeType$ Creature"
	svars := map[string]string{"DBA": line}
	root := &cards.SA{Kind: "SP", API: "Pump", Params: map[string]string{"ValidTgts": "Player"}, Sub: cards.ResolveSVar(svars, "DBA")}
	if root.Sub == nil {
		t.Fatal("sub-ability did not resolve")
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "G", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{root}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	for _, r := range lowerCard(p, "Synthetic", card, false).Refusals {
		if strings.Contains(r.Reason, "without IsRemembered") {
			return
		}
	}
	t.Fatal("dual-origin ChangeZoneAll with unrestricted ChangeType was not refused")
}

// TestDualOriginDefinedStaysUnread: node()'s Defined$ Targeted.<q> check peeks at
// Defined$; a node whose effect ignores its Defined$ must still refuse it as
// unread (Mana Crypt's FlipCoin Defined$ You), not lower it silently.
func TestDualOriginDefinedStaysUnread(t *testing.T) {
	face := &cards.Face{Name: "Synthetic", ManaCost: "G", Types: []string{"Sorcery"},
		Abilities: []*cards.SA{{Kind: "SP", API: "FlipCoin", Params: map[string]string{"Defined": "You"}}}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	for _, r := range lowerCard(p, "Synthetic", card, false).Refusals {
		if strings.Contains(r.Reason, "unread params [Defined]") {
			return
		}
	}
	t.Fatal("FlipCoin Defined$ You was not refused as unread")
}
