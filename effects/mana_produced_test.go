package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
)

// TestManaUnparsedComboFailsClosed pins effMana's handling of a Produced$
// Combo R G reaching the primitive (from a path with no colour chooser). The
// original bug was a raw rune walk -- C,o,m,b,o,R,G -- adding five stray
// colourless plus a red and a green; the head must be parsed, never walked.
// The parse then applies the DEGENERATE but real reading: the full amount in
// EVERY listed colour (the colour-combination ask is the M4 mana-choice
// milestone), so Burnt Offering's Amount$ X resolves instead of hard-failing
// to zero mana. Chosen/ChosenColor shapes keep the fail-closed note (the
// next test): they have no degenerate reading at all.
func TestManaUnparsedComboFailsClosed(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Source: 0, Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ Combo R G"))
	var red, green int32
	noteFound := false
	for _, ev := range h.log {
		if ev.Kind == events.ManaAdd && ev.Counter == "R" {
			red += ev.Amount
		}
		if ev.Kind == events.ManaAdd && ev.Counter == "G" {
			green += ev.Amount
		}
		if ev.Kind == events.Note && strings.Contains(ev.Text, "Combo R G") {
			noteFound = true
		}
	}
	if red != 1 || green != 1 {
		t.Fatalf("Combo R G degenerate output red=%d green=%d, want 1 each (amount 1 per listed colour)", red, green)
	}
	if noteFound {
		t.Fatalf("Combo R G still recorded an unhandled-Produced$ note: %+v", h.log)
	}
}

// TestManaUnparsedChosenFailsClosed covers the other non-literal Produced$
// family a card can carry (Chosen / ChosenColor), which also must not be
// walked as garbage.
func TestManaUnparsedChosenFailsClosed(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Source: 0, Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ ChosenColor"))
	for _, ev := range h.log {
		if ev.Kind == events.ManaAdd {
			t.Fatalf("unparsed ChosenColor emitted a ManaAdd: %+v", ev)
		}
	}
}

// TestManaLiteralRRStillAddsTwoRed pins the rune walk is not lost: an
// unresolvable-into-garbage change must preserve the legitimate
// multi-symbol literal.
func TestManaLiteralRRStillAddsTwoRed(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Source: 0, Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ RR"))
	red := 0
	for _, ev := range h.log {
		if ev.Kind == events.ManaAdd && ev.Counter == "R" {
			red++
		}
	}
	if red != 2 {
		t.Fatalf("Produced$ RR added %d red, want 2", red)
	}
}

// TestManaLiteralWUAddsEachMixedSymbol pins a mixed-colour literal in the
// same rune walk.
func TestManaLiteralWUAddsEachMixedSymbol(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Source: 0, Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ W U"))
	got := map[string]int{}
	for _, ev := range h.log {
		if ev.Kind == events.ManaAdd {
			got[ev.Counter]++
		}
	}
	if got["W"] != 1 || got["U"] != 1 {
		t.Fatalf("Produced$ W U added %v, want one W and one U", got)
	}
}
