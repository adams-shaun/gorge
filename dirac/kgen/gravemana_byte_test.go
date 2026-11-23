package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// graveManaSynth lowers a synthetic artifact whose one ability is line.
func graveManaSynth(t *testing.T, line string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("gravemana_probe.txt", []byte("Name:Grave Mana Byte Probe\nManaCost:0\nTypes:Artifact\n"+line+"\n"))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	const name = "Grave Mana Byte Probe"
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, name, card, false)
}

// TestGraveManaLowersByte: Jack-o'-Lantern's "{1}, Exile CARDNAME from your
// graveyard: Add one mana of any color" sets CardDef's GraveMana (lCard
// GraveMana, the colours it adds: any) and lowers with no refusal: the line
// lowers onto the byte (dirac/mana_gravemana.go reads it), not into a mana
// ability. A coloured fee or another amount is not the shape: no byte.
func TestGraveManaLowersByte(t *testing.T) {
	jack := exileGraveLower(t, "Jack-o'-Lantern")
	if jack.GraveMana != uint8(kop.ColAny) {
		t.Errorf("Jack-o'-Lantern: GraveMana %#x, want %#x", jack.GraveMana, uint8(kop.ColAny))
	}
	if got := keclReason(jack.Refusals); got != "" {
		t.Errorf("Jack-o'-Lantern: refusals %q, want none", got)
	}

	const line = "A:AB$ Mana | Cost$ %s | ActivationZone$ Graveyard | Produced$ Any%s"
	for _, c := range []struct {
		name, cost, extra string
		want              bool
	}{
		{"the shape", "1 ExileFromGrave<1/CARDNAME>", "", true},
		{"a coloured fee", "G ExileFromGrave<1/CARDNAME>", "", false},
		{"two generic", "2 ExileFromGrave<1/CARDNAME>", "", false},
		{"no fee", "ExileFromGrave<1/CARDNAME>", "", false},
		{"another amount", "1 ExileFromGrave<1/CARDNAME>", " | Amount$ 2", false},
		{"a restriction", "1 ExileFromGrave<1/CARDNAME>", " | RestrictValid$ Card", false},
		{"a rider", "1 ExileFromGrave<1/CARDNAME>", " | TriggersWhenSpent$ DBX", false},
	} {
		src := strings.Replace(strings.Replace(line, "%s", c.cost, 1), "%s", c.extra, 1)
		lc := graveManaSynth(t, src)
		if got := lc.GraveMana != 0; got != c.want {
			t.Errorf("%s: GraveMana %#x, want a byte = %v", c.name, lc.GraveMana, c.want)
		}
	}
}
