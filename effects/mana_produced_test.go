package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
)

// TestManaUnparsedComboFailsClosed pins effMana's fail-closed guard on the
// exact bug shape: a Produced$ Combo R G reaching the primitive (from a path
// with no colour chooser) must emit NO ManaAdd and instead record a Note
// naming the unhandled value. Before the fix this value was walked one rune
// at a time -- C,o,m,b,o,R,G -- adding five stray colourless plus a red and
// a green.
func TestManaUnparsedComboFailsClosed(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Source: 0, Controller: 0}
	Resolve(h, c, sa(t, "AB$ Mana | Cost$ T | Produced$ Combo R G"))
	noteFound := false
	for _, ev := range h.log {
		if ev.Kind == events.ManaAdd {
			t.Fatalf("unparsed Combo R G emitted a ManaAdd: %+v", ev)
		}
		if ev.Kind == events.Note && strings.Contains(ev.Text, "Combo R G") {
			noteFound = true
		}
	}
	if !noteFound {
		t.Fatalf("no Note naming the unhandled Produced$ value: %+v", h.log)
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
