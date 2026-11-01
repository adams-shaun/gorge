package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// komenLower lowers a synthetic Omen card: front is the lines after PT:,
// face the inset spell's script body. token lowers it as a token script.
func komenLower(t *testing.T, front, face string, token bool) []refusal {
	t.Helper()
	src := "Name:Omen Probe\nManaCost:1 R\n" + front + "\nAlternateMode:Omen\n\nALTERNATE\n\nName:Omen Spell\nManaCost:R\n" + face + "\n"
	card, diags := cards.ParseBytes("omen_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Omen Probe"}, byName: map[string]*cards.Card{"Omen Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Omen Probe", card, token).Refusals
}

// TestOmenRefusals: an Omen card lowers only in gorge's omenSpellFace
// shape (face 1 an Instant or Sorcery of type Omen, a nonland front, a
// pool card); every other shape is refused with a named reason.
func TestOmenRefusals(t *testing.T) {
	const (
		frontCreature = "Types:Creature Human\nPT:1/1"
		frontLand     = "Types:Land"
		faceOmen      = "Types:Instant Omen\nA:SP$ GainLife | LifeAmount$ 1"
	)
	if r := komenLower(t, frontCreature, faceOmen, false); len(r) != 0 {
		t.Fatalf("an accepted shape is refused: %v", r)
	}
	cases := []struct {
		name, front, face string
		token             bool
		want              string
	}{
		{"face is a creature", frontCreature, "Types:Creature Omen\nPT:1/1", false, "Omen face 1 is not an instant or sorcery of type Omen"},
		{"face is not an Omen", frontCreature, "Types:Instant\nA:SP$ GainLife | LifeAmount$ 1", false, "Omen face 1 is not an instant or sorcery of type Omen"},
		{"land front", frontLand, faceOmen, false, "Omen card with a land front"},
		{"token script", frontCreature, faceOmen, true, "Omen layout on a token script"},
	}
	for _, c := range cases {
		var got []string
		for _, x := range komenLower(t, c.front, c.face, c.token) {
			got = append(got, x.Reason)
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, c.want) {
			t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
		}
	}
}
