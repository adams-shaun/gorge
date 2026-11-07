package kgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// mzLower lowers one synthetic card, as abLower does.
func mzLower(t *testing.T, src string) *lCard {
	t.Helper()
	card, diags := cards.ParseBytes("Probe.txt", []byte(src))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card},
		nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerPool(p).cards[0]
}

func mzMayPlays(c *lCard) []lInstr {
	var out []lInstr
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "MayPlay" {
				out = append(out, in)
			}
		}
	}
	return out
}

func mzReasons(c *lCard) string {
	var rs []string
	for _, r := range c.Refusals {
		rs = append(rs, r.Reason)
	}
	return strings.Join(rs, "; ")
}

// TestEffectMayPlayZoneListLowers: the remembered-card MayPlay static accepts
// the Graveyard,Exile list only with the same ExileOnMoved$ list, emitting one
// MayPlay instruction per zone with identical flags; every other zone list
// refuses by name.
func TestEffectMayPlayZoneListLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte("Ark of Hunger\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	ark := lowerPool(p).cards[0]
	if got := mzReasons(ark); got != "" {
		t.Fatalf("Ark of Hunger refused: %s", got)
	}
	got := mzMayPlays(ark)
	if len(got) != 2 {
		t.Fatalf("Ark of Hunger lowered to %d MayPlay instructions (%v), want one per zone", len(got), got)
	}
	if got[0].C != zGraveyard || got[1].C != zExile {
		t.Fatalf("MayPlay zones %d,%d, want graveyard then exile", got[0].C, got[1].C)
	}
	if got[0].B.Kind != got[1].B.Kind || got[0].B.N != got[1].B.N {
		t.Fatalf("MayPlay flags differ: %v vs %v", got[0].B, got[1].B)
	}
	if got[0].B.N&kop.MayPlayExileOnMoved == 0 {
		t.Fatalf("MayPlay flags %d lack the ExileOnMoved lifetime", got[0].B.N)
	}

	const head = "Name:Probe\nManaCost:1 U\nTypes:Instant\n" +
		"A:SP$ Effect | RememberObjects$ RememberedCard | StaticAbilities$ Play | ExileOnMoved$ %s | SpellDescription$ Probe.\n" +
		"SVar:Play:Mode$ Continuous | MayPlay$ True | EffectZone$ Command | Affected$ Card.IsRemembered | AffectedZone$ %s | Description$ Probe.\n"

	unknown := mzLower(t, fmt.Sprintf(head, "Graveyard,Hand", "Graveyard,Hand"))
	if !strings.Contains(mzReasons(unknown), `AffectedZone "Graveyard,Hand"`) {
		t.Fatalf("unknown zone list not refused by name: %q", mzReasons(unknown))
	}

	mism := mzLower(t, fmt.Sprintf(head, "Graveyard", "Graveyard,Exile"))
	if !strings.Contains(mzReasons(mism), `ExileOnMoved "Graveyard"`) {
		t.Fatalf("ExileOnMoved mismatch not refused by name: %q", mzReasons(mism))
	}
}
