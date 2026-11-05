package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// otherEachRefusals lowers a synthetic sorcery exiling up to one target per other
// player over filter vt (TargetMax$ OneEach, SVar OneEach PlayerCountOther$Amount).
func otherEachRefusals(t *testing.T, vt string) []string {
	t.Helper()
	svars := map[string]string{"OneEach": "PlayerCountOther$Amount"}
	root := "SP$ ChangeZone | ValidTgts$ " + vt + " | TargetMin$ 0 | TargetMax$ OneEach | TargetsForEachPlayer$ True | Origin$ Battlefield | Destination$ Exile"
	r := cards.ResolveSVar(map[string]string{"R": root}, "R")
	if r == nil {
		t.Fatalf("root %q did not resolve", root)
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "B", Types: []string{"Sorcery"}, SVars: svars, Abilities: []*cards.SA{r}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	var out []string
	for _, rf := range lowerCard(p, "Synthetic", card, false).Refusals {
		out = append(out, rf.Reason)
	}
	return out
}

// TestOtherEachShapeOnly: the PlayerCountOther$ bounds are set only for the
// opponent-permanent filter; any other filter still refuses at the value read,
// with the message it had before the shape was accepted.
func TestOtherEachShapeOnly(t *testing.T) {
	if got := otherEachRefusals(t, "Creature.YouDontCtrl"); len(got) != 0 {
		t.Fatalf("accepted shape refused: %q", got)
	}
	got := otherEachRefusals(t, "Creature.YouCtrl")
	if len(got) == 0 {
		t.Fatal("a YouCtrl per-player target lowered")
	}
	want := `SVar OneEach="PlayerCountOther$Amount": value head PlayerCountOther$`
	if !strings.Contains(strings.Join(got, "\n"), want) {
		t.Errorf("refusals %q lack %q", got, want)
	}
}
