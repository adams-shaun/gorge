package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPumpUntilYourNextUpkeepLowers: Erhnam Djinn's Pump Duration$
// UntilYourNextUpkeep lowers to the kernel's landwalk override record with
// the UntilUpkeep class flag. A non-landwalk keyword and a P/T pump with the
// same duration keep the existing Pump Duration refusal.
func TestPumpUntilYourNextUpkeepLowers(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Erhnam Djinn")
	if !ok || card == nil {
		t.Fatal("Erhnam Djinn not in the corpus")
	}
	name := card.Faces[0].Name
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: reg}
	lw := lowerPool(p)
	if len(lw.cards) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(lw.cards))
	}
	c := lw.cards[0]
	if len(c.Refusals) != 0 {
		t.Fatalf("%s refused: %v", c.Name, c.Refusals)
	}

	n := 0
	for _, ab := range c.Abilities {
		for _, in := range ab.Prog {
			if in.Op != "Override" || in.CS != "UntilYourNextUpkeep" {
				continue
			}
			n++
			if in.C < 0 || in.C >= len(c.overrides) {
				t.Fatalf("Override CS=UntilYourNextUpkeep names row %d of %d", in.C, len(c.overrides))
			}
			o := c.overrides[in.C]
			if !o.UntilUpkeep {
				t.Errorf("override row %d: UntilUpkeep false", in.C)
			}
			if o.Landwalk == nil {
				t.Errorf("override row %d: Landwalk nil, want the Forest filter", in.C)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%s lowers %d Override CS=UntilYourNextUpkeep instructions, want 1", c.Name, n)
	}

	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	if len(bs) != 1 {
		t.Fatalf("built %d cards, want 1", len(bs))
	}
	if len(bs[0].refusals) != 0 {
		t.Fatalf("%s build refusals: %v", c.Name, bs[0].refusals)
	}
	found := false
	for i := range pt.overrides {
		o := &pt.overrides[i]
		if !o.UntilUpkeep {
			continue
		}
		found = true
		if o.Landwalk == 0 {
			t.Errorf("built override row %d: UntilUpkeep true with no Landwalk filter", i)
		}
	}
	if !found {
		t.Fatalf("no built override row carries UntilUpkeep (rows %+v)", pt.overrides)
	}

	const base = "Name:Upkeep Probe\nManaCost:1 G\nTypes:Instant\n" +
		"A:SP$ Pump | ValidTgts$ Creature.nonWall+OppCtrl | KW$ %s | Duration$ UntilYourNextUpkeep%s"
	for _, tc := range []struct{ name, kw, extra string }{
		{"non-landwalk keyword", "Flying", ""},
		{"P/T pump", "Landwalk:Forest", " | NumAtt$ +1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := keclReason(keclSynth(t, "Upkeep Probe", fmt.Sprintf(base, tc.kw, tc.extra)))
			if !strings.Contains(got, "Pump Duration") {
				t.Errorf("refusal %q, want one naming Pump Duration", got)
			}
		})
	}
}
